package network

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"

	"github.com/mcfx/tcoin/core/block"
)

func TestSerializationBlockRequest(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	p := PacketBlockRequest{
		MinId: rnd.Intn(1919810),
	}
	rnd.Read(p.Hash[:])
	var b bytes.Buffer
	err := EncodeBlockRequest(&b, p)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := DecodeBlockRequest(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Fatal("not equal")
	}
}

func TestSerializationBlocks(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	p := NewPakcetBlocks(1919810)
	blk := &block.Block{
		Header: block.BlockHeader{
			ParentHash: block.HashType{1, 2, 4},
			ExtraData:  block.HashType{1, 2, 5},
		},
		Miner: block.AddressType{1, 2, 6},
		Time:  127,
		Txs:   []*block.Transaction{},
	}
	blk.FillHash()
	for i := 0; i < 75; i++ {
		if rnd.Intn(2) == 1 {
			p.Add(blk, true)
		} else {
			p.Add(blk.Header, false)
		}
	}
	var b bytes.Buffer
	err := EncodeBlocks(&b, p)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := DecodeBlocks(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Fatal("not equal")
	}
}

func TestSerializationTransactions(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	tx := &block.Transaction{
		TxType:   1,
		Value:    rnd.Uint64(),
		GasLimit: rnd.Uint64(),
		Fee:      rnd.Uint64(),
		Nonce:    rnd.Uint64(),
		Data:     []byte{1, 2, 3},
	}
	rnd.Read(tx.SenderPubkey[:])
	rnd.Read(tx.SenderSig[:])
	rnd.Read(tx.Receiver[:])
	p := PacketTransactions{
		Txs: make([]*block.Transaction, 0),
	}
	for i := 0; i < 75; i++ {
		p.Txs = append(p.Txs, tx)
	}
	var b bytes.Buffer
	err := EncodeTransactions(&b, p)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := DecodeTransactions(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Fatal("not equal")
	}
}
