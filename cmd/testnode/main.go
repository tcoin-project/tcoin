package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"flag"
	"log"
	"math/rand"
	"time"

	"github.com/mcfx/tcoin/core"
	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/core/consensus"
	"github.com/mcfx/tcoin/utils/corerpc"
)

func main() {
	path := flag.String("path", "", "storage path")
	rpcAddr := flag.String("rpc", "", "rpc listen addr")
	flag.Parse()
	if *path == "" {
		log.Fatal("path can't be empty")
	}
	if *rpcAddr == "" {
		log.Fatal("rpc can't be empty")
	}
	rnd := rand.New(rand.NewSource(114514))
	pubkey, privkey := block.GenKeyPair(rnd)
	log.Printf("miner private key: %x\n", ed25519.PrivateKey(privkey[:]).Seed())
	c := core.ChainNodeConfig{
		StoragePath:          *path,
		StorageFinalizeDepth: 5,
		StorageDumpDiskRatio: 0.01,
		ListenPort:           -1,
		MaxConnections:       0,
	}
	lastTime := uint64(time.Now().UnixNano())
	b := &block.Block{
		Header: block.BlockHeader{
			ParentHash: block.HashType{},
			ExtraData:  block.HashType{},
		},
		Miner: block.PubkeyToAddress(pubkey),
		Time:  lastTime,
		Txs:   []*block.Transaction{},
	}
	for i := 0; i < block.HashLen; i++ {
		b.Header.ParentHash[i] = 0xcc
	}
	b.FillHash()
	df := block.HashType{1}
	for i := 0; bytes.Compare(b.Header.Hash[:], df[:]) > 0; i++ {
		binary.LittleEndian.PutUint64(b.Header.ExtraData[:8], uint64(i))
		b.Header.Hash = b.Header.ComputeHash()
	}
	var bi uint64 = 1000000000
	gc := core.ChainGlobalConfig{
		ChainId:      4,
		GenesisBlock: b,
		GenesisConsensusState: &consensus.ConsensusState{
			Height:           -1,
			LastBlockTime:    0,
			LastKeyBlockTime: 0,
			Difficulty:       df,
		},
		GenesisBlockReward: bi * 10000000,
		BlockReward:        bi,
	}
	ec := core.ExplorerExecutionCallback()
	n, err := core.NewChainNode(c, gc, ec)
	if err != nil {
		log.Fatalf("failed to set up node: %v", err)
	}
	rpc := corerpc.NewServer(n)
	go rpc.Run(*rpcAddr)
	go n.Run()
	for {
		du := time.Until(time.Unix(0, int64(lastTime+10*bi)))
		if du > 0 {
			time.Sleep(du)
		}
		b := n.GetBlockCandidate(block.PubkeyToAddress(pubkey))
		_, cs, _ := n.GetHighest()
		for i := 0; bytes.Compare(b.Header.Hash[:], cs.Difficulty[:]) > 0; i++ {
			binary.LittleEndian.PutUint64(b.Header.ExtraData[:8], uint64(i))
			b.Header.Hash = b.Header.ComputeHash()
		}
		n.SubmitBlock(b)
		lastTime = b.Time
	}
}
