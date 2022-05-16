package consensus

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"

	"github.com/mcfx/tcoin/core/block"
)

func TestConsensusSerialization(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	cs := &ConsensusState{
		Height:           rnd.Int(),
		LastBlockTime:    rnd.Uint64(),
		LastKeyBlockTime: rnd.Uint64(),
	}
	rnd.Read(cs.Difficulty[:])

	var b bytes.Buffer
	err := EncodeConsensus(&b, cs)
	if err != nil {
		t.Fatal(err)
	}
	cs2, err := DecodeConsensus(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cs, cs2) {
		t.Fatal("not equal")
	}
}

func TestConsensus(t *testing.T) {
	cs := &ConsensusState{
		Height:           0,
		LastBlockTime:    0,
		LastKeyBlockTime: 0,
		Difficulty:       block.HashType{1},
	}
	blk := &block.Block{
		Header: block.BlockHeader{
			ParentHash: block.HashType{1},
			ExtraData:  block.HashType{1},
		},
		Miner: block.AddressType{1},
		Time:  0,
		Txs:   []*block.Transaction{},
	}
	var lstTime uint64 = 0
	for i := 1; i <= 3000; i++ {
		lstTime += 1000000000 * 8
		blk.Time = lstTime
		if !cs.CheckAndUpdate(blk) {
			t.Fatal("block rejected")
		}
	}
	if cs.Difficulty[1] != 0 {
		t.Fatal("state difficulty invalid")
	}
	for i := 1; i <= 3000; i++ {
		lstTime += 1000000000 * 12
		blk.Time = lstTime
		if !cs.CheckAndUpdate(blk) {
			t.Fatal("block rejected")
		}
	}
	dd := cs.Difficulty
	if dd[1] < 0xa0 {
		t.Fatal("state difficulty invalid")
	}
	for i := 1; i <= 3000; i++ {
		lstTime += 1000000000 * 10
		blk.Time = lstTime
		if !cs.CheckAndUpdate(blk) {
			t.Fatal("block rejected")
		}
	}
	if dd != cs.Difficulty {
		t.Fatal("state difficulty invalid")
	}
}
