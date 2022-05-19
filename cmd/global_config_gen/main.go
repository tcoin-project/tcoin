package main

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"os"
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
	var bi uint64 = 1000000000
	gConfig := core.ChainGlobalConfig{
		ChainId:      8888,
		GenesisBlock: b,
		GenesisConsensusState: &consensus.ConsensusState{
			Height:           -1,
			LastBlockTime:    0,
			LastKeyBlockTime: 0,
			Difficulty:       block.HashType{0xff},
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
	err = os.WriteFile("global_config.json", bu, 0o755)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("pubkey: %x", pub[:])
	log.Printf("privkey: %x", priv[:])
}
