package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"log"
	"time"

	"github.com/mcfx/tcoin/core"
	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/core/consensus"
)

func main() {
	pub, priv := block.GenKeyPair(rand.Reader)
	b := &block.Block{
		Header: block.BlockHeader{
			ParentHash: block.HashType{},
			ExtraData:  block.HashType{},
		},
		Miner: block.PubkeyToAddress(pub),
		Time:  uint64(time.Now().UnixNano()),
		Txs:   []*block.Transaction{},
	}
	for i := 0; i < block.HashLen; i++ {
		b.Header.ParentHash[i] = 0x22
	}
	b.FillHash()
	df := block.HashType{0, 0, 0, 0xf}
	for i := 0; bytes.Compare(b.Header.Hash[:], df[:]) > 0; i++ {
		binary.LittleEndian.PutUint64(b.Header.ExtraData[:8], uint64(i))
		b.Header.Hash = b.Header.ComputeHash()
	}
	var bi uint64 = 1000000000
	gConfig := core.ChainGlobalConfig{
		ChainId:      8888,
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
	bu, err := json.Marshal(gConfig)
	if err != nil {
		log.Fatal(err)
	}
	var gc2 core.ChainGlobalConfig
	err = json.Unmarshal(bu, &gc2)
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile("global_config.json", bu, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("pubkey: %x", pub[:])
	log.Printf("privkey: %x", priv[:])
}
