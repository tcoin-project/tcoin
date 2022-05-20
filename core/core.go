package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/core/consensus"
	cnet "github.com/mcfx/tcoin/core/network"
	"github.com/mcfx/tcoin/network"
	"github.com/mcfx/tcoin/storage"

	"github.com/patrickmn/go-cache"
)

type ChainNode struct {
	se                  *storage.StorageEngine
	unresolvedBlocks    *cache.Cache
	blockCache          *cache.Cache
	blockConsensusState *cache.Cache
	txPool              *cache.Cache
	neighborHeight      *cache.Cache
	possibleNext        *cache.Cache
	nc                  *network.Client
	rchan               chan network.ClientPacket
	txChan              chan *block.Transaction
	checkUnBlocks       chan bool
	broadcastBlocks     chan bool
	config              ChainNodeConfig
	gConfig             ChainGlobalConfig
	ecxt                *block.ExecutionContext
	stop                chan bool
	stopped             chan bool
	seMut               sync.Mutex
}

func NewChainNode(config ChainNodeConfig, gConfig ChainGlobalConfig, ecxt *block.ExecutionContext) (*ChainNode, error) {
	if gConfig.GenesisBlock.Header.ComputeHash() != gConfig.GenesisBlock.Header.Hash {
		return nil, errors.New("failed to init node: header hash mismatch")
	}
	if gConfig.GenesisBlock.ComputeHash() != gConfig.GenesisBlock.Header.BodyHash {
		return nil, errors.New("failed to init node: header hash mismatch")
	}
	sl := storage.EmptySlice()
	err := block.ExecuteBlock(gConfig.GenesisBlock, gConfig.GenesisBlockReward, sl, ecxt)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	cs := gConfig.GenesisConsensusState
	if !cs.CheckAndUpdate(gConfig.GenesisBlock) {
		return nil, errors.New("failed to init node: consensus rejected")
	}
	var buf bytes.Buffer
	err = consensus.EncodeConsensus(&buf, cs)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	err = block.EncodeBlock(&buf, gConfig.GenesisBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	se, err := storage.NewStorageEngine(storage.StorageEngineConfig{
		FinalizeDepth: config.StorageFinalizeDepth,
		DumpDiskRatio: config.StorageDumpDiskRatio,
		Path:          config.StoragePath,
	}, sl, storage.SliceKeyType(gConfig.GenesisBlock.Header.Hash), buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	rchan := make(chan network.ClientPacket, 10000)
	nc, err := network.NewClient(&network.ClientConfig{
		Port:           config.ListenPort,
		MaxConnections: config.MaxConnections,
		Path:           config.StoragePath,
	}, rchan, gConfig.ChainId)
	if err != nil {
		return nil, fmt.Errorf("failed to init node: %v", err)
	}
	if len(gConfig.SeedNodes) > 0 {
		nc.AddPeers(gConfig.SeedNodes)
	}
	cn := &ChainNode{
		se:                  se,
		unresolvedBlocks:    cache.New(time.Minute*5, time.Minute*10),
		blockCache:          cache.New(time.Minute*5, time.Minute*10),
		blockConsensusState: cache.New(time.Minute*5, time.Minute*10),
		txPool:              cache.New(time.Minute*5, time.Minute*10),
		neighborHeight:      cache.New(time.Minute*5, time.Minute*10),
		possibleNext:        cache.New(time.Minute*5, time.Minute*10),
		nc:                  nc,
		rchan:               rchan,
		txChan:              make(chan *block.Transaction, 100),
		checkUnBlocks:       make(chan bool, 10),
		broadcastBlocks:     make(chan bool, 10),
		config:              config,
		gConfig:             gConfig,
		ecxt:                ecxt,
		stop:                make(chan bool, 50),
		stopped:             make(chan bool, 10),
	}
	return cn, nil
}

func (cn *ChainNode) Stop() {
	cn.istop()
	for i := 0; i < 10; i++ {
		<-cn.stopped
	}
}

func (cn *ChainNode) istop() {
	for i := 0; i < 5; i++ {
		cn.stop <- true
	}
	cn.stopped <- true
}

func (cn *ChainNode) Run() {
	defer cn.istop()
	for i := 0; i < 4; i++ {
		go cn.readLoop()
	}
	go cn.broadcastTxsLoop()
	go cn.checkUnresolvedBlocks()
	go cn.syncLoop()
	go cn.sendMyHighest()
	for {
		slp := time.After(time.Second * 10)
		select {
		case <-slp:
		case <-cn.stop:
			return
		}
		c1, c2 := cn.nc.GetPeerCount()
		b, c, err := cn.GetHighest()
		if err != nil {
			log.Printf("nodes: %d (%d active) height error: %v", c1, c2, err)
		} else {
			log.Printf("nodes: %d (%d active) height: %d (%x)", c1, c2, c.Height, b.Header.Hash[:])
		}
	}
}

func (cn *ChainNode) readLoop() {
	defer cn.istop()
	for {
		var cp network.ClientPacket
		select {
		case cp = <-cn.rchan:
		case <-cn.stop:
			return
		}
		err := func() error {
			opcode := cp.Data[0]
			data := cp.Data[1:]
			buf := bytes.NewBuffer(data)
			if opcode == cnet.PktBlockRequest {
				p, err := cnet.DecodeBlockRequest(buf)
				if err != nil {
					return err
				}
				return cn.handleBlockRequest(p, cp.PeerId, 10)
			} else if opcode == cnet.PktBlocks {
				p, err := cnet.DecodeBlocks(buf)
				if err != nil {
					return err
				}
				if p.MinId != -1 {
					tmp := make([]byte, 8)
					binary.LittleEndian.PutUint64(tmp, uint64(cp.PeerId))
					t, ok := cn.neighborHeight.Get(string(tmp))
					np := p.MinId + len(p.Body) - 1
					if ok {
						if t.(int) > np {
							np = t.(int)
						}
					}
					cn.neighborHeight.Set(string(tmp), np, cache.DefaultExpiration)
				}
				return cn.handleBlocks(p)
			} else if opcode == cnet.PktTransactions {
				p, err := cnet.DecodeTransactions(buf)
				if err != nil {
					return err
				}
				return cn.handleTransactions(p)
			}
			return nil
		}()
		if err != nil {
			cn.nc.DiscardPeer(cp.PeerId, time.Minute*10)
		}
	}
}

func (cn *ChainNode) loadBlock(height int, hash block.HashType) (*block.Block, *consensus.ConsensusState, error) {
	cn.seMut.Lock()
	defer cn.seMut.Unlock()
	s, err := cn.se.ReadData(height, storage.SliceKeyType(hash))
	if err != nil {
		return nil, nil, err
	}
	buf := bytes.NewBuffer(s)
	cs, err := consensus.DecodeConsensus(buf)
	if err != nil {
		return nil, nil, err
	}
	b, err := block.DecodeBlock(buf)
	if err != nil {
		return nil, nil, err
	}
	if (block.HashType{}) == hash {
		hash = b.Header.Hash
	} else if b.Header.Hash != hash {
		return nil, nil, errors.New("block hash mismatch, possibly wrong height")
	}
	cn.blockConsensusState.Set(string(hash[:]), cs, cache.DefaultExpiration)
	cn.blockCache.Set(string(hash[:]), b, cache.DefaultExpiration)
	return b, cs, nil
}

func (cn *ChainNode) getBlock(height int, hash block.HashType) (*block.Block, error) {
	t, ok := cn.blockCache.Get(string(hash[:]))
	if ok {
		return t.(*block.Block), nil
	}
	b, _, err := cn.loadBlock(height, hash)
	return b, err
}

func (cn *ChainNode) getConsensusState(height int, hash block.HashType) (*consensus.ConsensusState, error) {
	t, ok := cn.blockConsensusState.Get(string(hash[:]))
	if ok {
		return t.(*consensus.ConsensusState), nil
	}
	_, c, err := cn.loadBlock(height, hash)
	return c, err
}

func (cn *ChainNode) handleBlockRequest(p cnet.PacketBlockRequest, peerId, maxReturn int) error {
	cn.seMut.Lock()
	hs := cn.se.HighestSlice
	hc := cn.se.HighestChain
	cn.seMut.Unlock()
	rp, err := func() (cnet.PacketBlocks, error) {
		rp := cnet.NewPacketBlocks(0)
		if p.MinId == -1 {
			b, err := cn.getBlock(p.MinId, p.Hash)
			if err != nil {
				return rp, err
			}
			cs, err := cn.getConsensusState(p.MinId, p.Hash)
			if err != nil {
				return rp, err
			}
			rp.MinId = cs.Height
			rp.Add(b, true)
			return rp, nil
		}
		if p.MinId > hs.Height() {
			return rp, errors.New("too high, no such block")
		}
		mh := hc[0].S.Height()
		if p.MinId >= mh && hc[p.MinId-mh].Key != p.Hash && p.Hash != (block.HashType{}) {
			return rp, fmt.Errorf("invalid chain: id=%d mh=%d hs=%x my=%x", p.MinId, mh, p.Hash[:], hc[p.MinId-mh].Key[:])
		}
		rp.MinId = p.MinId
		for i := 0; i < maxReturn; i++ {
			h := block.HashType{}
			if i == 0 && p.Hash != (block.HashType{}) {
				h = p.Hash
			} else if p.MinId+i-mh >= len(hc) {
				break
			} else if p.MinId+i >= mh {
				h = block.HashType(hc[p.MinId+i-mh].Key)
			}
			b, err := cn.getBlock(p.MinId+i, h)
			if err != nil {
				return rp, fmt.Errorf("getblock error: %v", err)
			}
			rp.Add(b, true)
		}
		return rp, nil
	}()
	if err != nil {
		log.Printf("handleBlockRequest error: %v", err)
		rp = cnet.NewPacketBlocks(hs.Height())
		b, err := cn.getBlock(hs.Height(), block.HashType(hc[len(hc)-1].Key))
		if err != nil {
			rp.Add(b.Header, false)
		}
	}
	var buf bytes.Buffer
	buf.WriteByte(cnet.PktBlocks)
	err = cnet.EncodeBlocks(&buf, rp)
	if err != nil {
		return err
	}
	cn.nc.WriteTo(peerId, buf.Bytes())
	return nil
}

func (cn *ChainNode) sendHighest(peerId int, hs *storage.Slice, hc []storage.SliceChain, broadcast bool) error {
	rp := cnet.NewPacketBlocks(hs.Height())
	b, err := cn.getBlock(hs.Height(), block.HashType(hc[len(hc)-1].Key))
	if err != nil {
		rp.Add(b.Header, false)
	}
	var buf bytes.Buffer
	buf.WriteByte(cnet.PktBlocks)
	err = cnet.EncodeBlocks(&buf, rp)
	if err != nil {
		return err
	}
	if broadcast {
		cn.nc.Broadcast(buf.Bytes(), cn.config.MaxConnections)
	} else {
		cn.nc.WriteTo(peerId, buf.Bytes())
	}
	return nil
}

func (cn *ChainNode) checkUnresolvedBlocks() {
	defer cn.istop()
	for {
		select {
		case <-cn.checkUnBlocks:
		case <-cn.stop:
			return
		}
		ti := cn.unresolvedBlocks.Items()
		vis := make(map[block.HashType]bool)
		var mark func(k block.HashType, dep int)
		ask := []block.HashType{}
		any := false
		mark = func(k block.HashType, dep int) {
			if dep > cn.config.StorageFinalizeDepth+3 {
				return
			}
			if _, ok := vis[k]; ok {
				return
			}
			vis[k] = true
			_, ok := cn.se.GetSlice(storage.SliceKeyType(k))
			if ok {
				cn.unresolvedBlocks.Delete(string(k[:]))
				return
			}
			t, ok := ti[string(k[:])]
			if !ok {
				ask = append(ask, k)
				return
			}
			bh := t.Object.(block.BlockHeader)
			//log.Printf("checkfa: %x %x", k[:], bh.ParentHash[:])
			mark(bh.ParentHash, dep+1)
			bt, ok := cn.blockCache.Get(string(k[:]))
			if !ok {
				return
			}
			b := bt.(*block.Block)
			if b.Time > math.MaxInt64 || time.Unix(0, int64(b.Time)).After(time.Now().Add(time.Minute)) {
				cn.unresolvedBlocks.Delete(string(k[:]))
				return
			}
			cs, err := cn.getConsensusState(0, bh.ParentHash)
			if err != nil {
				return
			}
			cs = cs.Copy()
			if !cs.CheckAndUpdate(b) {
				return
			}
			var buf bytes.Buffer
			err = consensus.EncodeConsensus(&buf, cs)
			if err != nil {
				return
			}
			err = block.EncodeBlock(&buf, b)
			if err != nil {
				return
			}
			cn.seMut.Lock()
			sl, ok := cn.se.GetSlice(storage.SliceKeyType(bh.ParentHash))
			if ok {
				sln := storage.ForkSlice(sl)
				err := block.ExecuteBlock(b, cn.gConfig.BlockReward, sln, cn.ecxt)
				if err == nil {
					sln.Freeze()
					cn.se.AddFreezedSlice(sln, storage.SliceKeyType(k), storage.SliceKeyType(bh.ParentHash), buf.Bytes())
					cn.unresolvedBlocks.Delete(string(k[:]))
					// log.Printf("add block: %x", b.Header.Hash[:])
					any = true
				}
			}
			cn.seMut.Unlock()
		}
		var tk block.HashType
		for k := range ti {
			copy(tk[:], []byte(k))
			mark(tk, 0)
		}
		if any {
			cn.broadcastBlocks <- true
		}
		for _, k := range ask {
			p := cnet.PacketBlockRequest{
				MinId: -1,
				Hash:  k,
			}
			var buf bytes.Buffer
			buf.WriteByte(cnet.PktBlockRequest)
			err := cnet.EncodeBlockRequest(&buf, p)
			if err == nil {
				cn.nc.Broadcast(buf.Bytes(), 3)
			}
		}
	}
}

func (cn *ChainNode) handleBlocks(p cnet.PacketBlocks) error {
	for i, bt := range p.Body {
		var bh block.BlockHeader
		if p.IsFull(i) {
			b := bt.(*block.Block)
			cn.blockCache.Set(string(b.Header.Hash[:]), b, cache.DefaultExpiration)
			bh = b.Header
		} else {
			bh = bt.(block.BlockHeader)
		}
		// log.Printf("get block %d %x", p.MinId+i, bh.Hash[:])
		cn.unresolvedBlocks.Set(string(bh.Hash[:]), bh, cache.DefaultExpiration)
		cn.possibleNext.Set(string(bh.ParentHash[:]), bh.Hash, cache.DefaultExpiration)
	}
	cn.checkUnBlocks <- true
	return nil
}

func (cn *ChainNode) broadcastTx(tx *block.Transaction) {
	cn.txChan <- tx
}

func (cn *ChainNode) broadcastTxsLoop() {
	defer cn.istop()
	for {
		txs := make([]*block.Transaction, 0)
	ofor:
		for {
			select {
			case tx := <-cn.txChan:
				if tx == nil {
					break ofor
				}
				txs = append(txs, tx)
			case <-cn.stop:
				return
			}
		}
		if len(txs) == 0 {
			continue
		}
		var buf bytes.Buffer
		buf.WriteByte(cnet.PktTransactions)
		err := cnet.EncodeTransactions(&buf, cnet.PacketTransactions{Txs: txs})
		if err != nil {
			continue
		}
		cn.nc.Broadcast(buf.Bytes(), 3)
	}
}

func (cn *ChainNode) handleTransactions(p cnet.PacketTransactions) error {
	for _, tx := range p.Txs {
		hs := tx.Hash()
		err := cn.txPool.Add(string(hs[:]), tx, cache.DefaultExpiration)
		if err == nil {
			cn.broadcastTx(tx)
		}
	}
	cn.broadcastTx(nil)
	return nil
}

func (cn *ChainNode) syncLoop() {
	defer cn.istop()
	for {
		slp := time.After(time.Second * 5)
		select {
		case <-slp:
		case <-cn.broadcastBlocks:
		case <-cn.stop:
			return
		}
		cn.seMut.Lock()
		hs := cn.se.HighestSlice
		hc := cn.se.HighestChain
		cn.seMut.Unlock()
		mh := hc[len(hc)-1].S.Height()
		nh := cn.neighborHeight.Items()
		remReq := 2
		for k, v := range nh {
			id := int(binary.LittleEndian.Uint64([]byte(k)))
			h := v.Object.(int)
			if h < mh-3 {
				cn.sendHighest(id, hs, hc, false)
			} else if h < mh {
				if rand.Intn(4) == 0 {
					hs := block.HashType{}
					if h > hc[0].S.Height() {
						hs = block.HashType(hc[h+1-hc[0].S.Height()].Key)
					}
					cn.handleBlockRequest(cnet.PacketBlockRequest{
						MinId: h + 1,
						Hash:  hs,
					}, id, 5)
				}
			} else if h > mh {
				if rand.Intn(5) == 1 {
					cn.sendHighest(id, hs, hc, false)
				}
				if remReq > 0 {
					hs := block.HashType{}
					t, ok := cn.possibleNext.Get(string(hc[len(hc)-1].Key[:]))
					if ok {
						hs = t.(block.HashType)
					}
					p := cnet.PacketBlockRequest{
						MinId: mh + 1,
						Hash:  hs,
					}
					var buf bytes.Buffer
					buf.WriteByte(cnet.PktBlockRequest)
					err := cnet.EncodeBlockRequest(&buf, p)
					if err == nil {
						cn.nc.WriteTo(id, buf.Bytes())
						remReq--
					}
				}
			}
		}
	}
}

func (cn *ChainNode) sendMyHighest() {
	defer cn.istop()
	for {
		slp := time.After(time.Second * 30)
		select {
		case <-slp:
		case <-cn.stop:
			return
		}
		cn.seMut.Lock()
		hs := cn.se.HighestSlice
		hc := cn.se.HighestChain
		cn.seMut.Unlock()
		cn.sendHighest(0, hs, hc, true)
	}
}

func (cn *ChainNode) GetBlockCandidate(miner block.AddressType) *block.Block {
	// todo: sort by gas price
	txPool := cn.txPool.Items()
	cn.seMut.Lock()
	defer cn.seMut.Unlock()
	sl := storage.ForkSlice(cn.se.HighestSlice)
	b := &block.Block{}
	b.Header.ParentHash = block.HashType(cn.se.HighestChain[len(cn.se.HighestChain)-1].Key)
	b.Miner = miner
	b.Time = uint64(time.Now().UnixNano())
	b.Txs = make([]*block.Transaction, 0)
	for _, v := range txPool {
		tx := v.Object.(*block.Transaction)
		sl2 := storage.ForkSlice(sl)
		err := block.ExecuteTx(tx, sl2, cn.ecxt)
		if err == nil {
			sl2.Merge()
			b.Txs = append(b.Txs, tx)
		}
	}
	b.FillHash()
	return b
}

func (cn *ChainNode) SubmitBlock(b *block.Block) error {
	// log.Printf("submit block: %x", b.Header.Hash[:])
	cn.blockCache.Set(string(b.Header.Hash[:]), b, cache.DefaultExpiration)
	cn.unresolvedBlocks.Set(string(b.Header.Hash[:]), b.Header, cache.DefaultExpiration)
	cn.checkUnBlocks <- true
	p := cnet.NewPacketBlocks(-1)
	cs, err := cn.getConsensusState(-1, b.Header.ParentHash)
	if err == nil {
		p.MinId = cs.Height + 1
	}
	p.Add(b, true)
	var buf bytes.Buffer
	buf.WriteByte(cnet.PktBlocks)
	err = cnet.EncodeBlocks(&buf, p)
	if err != nil {
		return err
	}
	cn.nc.Broadcast(buf.Bytes(), 7)
	return nil
}

func (cn *ChainNode) GetHighest() (*block.Block, *consensus.ConsensusState, error) {
	cn.seMut.Lock()
	hc := cn.se.HighestChain
	cn.seMut.Unlock()
	uh := hc[len(hc)-1]
	h := block.HashType(uh.Key)
	b, err := cn.getBlock(uh.S.Height(), h)
	if err != nil {
		return nil, nil, err
	}
	c, err := cn.getConsensusState(uh.S.Height(), h)
	if err != nil {
		return nil, nil, err
	}
	return b, c, nil
}

func (cn *ChainNode) GetAccountInfo(addr block.AddressType) block.AccountInfo {
	cn.seMut.Lock()
	defer cn.seMut.Unlock()
	return block.GetAccountInfo(cn.se.HighestSlice, addr)
}

func (cn *ChainNode) SubmitTx(tx *block.Transaction) error {
	return cn.handleTransactions(cnet.PacketTransactions{
		Txs: []*block.Transaction{tx},
	})
}

func (cn *ChainNode) GetBlock(height int) (*block.Block, *consensus.ConsensusState, error) {
	cn.seMut.Lock()
	hc := cn.se.HighestChain
	cn.seMut.Unlock()
	mh := hc[len(hc)-1].S.Height()
	hash := block.HashType{}
	if height >= mh {
		hash = block.HashType(hc[height-hc[0].S.Height()].Key)
	}
	b, err := cn.getBlock(height, hash)
	if err != nil {
		return nil, nil, err
	}
	c, err := cn.getConsensusState(height, hash)
	if err != nil {
		return nil, nil, err
	}
	return b, c, err
}
