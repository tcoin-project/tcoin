package crypto

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"

	"github.com/mcfx/tcoin/storage"
)

func TestBlockSerialization(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	tx := &Transaction{
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

	blk := &Block{
		Header: BlockHeader{
			ParentHash: HashType{1, 2, 4},
			ExtraData:  HashType{1, 2, 5},
		},
		Miner:     AddressType{1, 2, 6},
		Timestamp: 127,
		Txs:       []*Transaction{tx, tx, tx},
	}
	blk.FillHash()

	var b bytes.Buffer
	err := EncodeBlock(&b, blk)
	if err != nil {
		t.Fatal(err)
	}
	bs := b.Bytes()
	blk2, err := DecodeBlock(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(blk, blk2) {
		t.Fatal("not equal")
	}
	bs[0] ^= 1
	b2 := bytes.NewBuffer(bs)
	_, err = DecodeBlock(b2)
	if err == nil || err.Error() != "block header hash mismatch" {
		t.Fatalf("expect fail, but returned %v", err)
	}
	bs[0] ^= 1
	bs[len(bs)-1] ^= 1
	b2 = bytes.NewBuffer(bs)
	_, err = DecodeBlock(b2)
	if err == nil || err.Error() != "block body hash mismatch" {
		t.Fatalf("expect fail, but returned %v", err)
	}
}

func TestBlockFeeExec(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	pubk1, prik1 := GenKeyPair(rnd)
	pubk2, _ := GenKeyPair(rnd)
	addr1 := PubkeyToAddress(pubk1)
	addr2 := PubkeyToAddress(pubk2)
	s := storage.EmptySlice()
	info := GetAccountInfo(s, addr1)
	info.Balance = 10000000
	SetAccountInfo(s, addr1, info)
	tx := &Transaction{
		TxType:       1,
		SenderPubkey: pubk1,
		Receiver:     addr2,
		Value:        500000,
		GasLimit:     100000,
		Fee:          100000,
		Nonce:        0,
		Data:         []byte{1, 2, 3},
	}
	tx.Sign(prik1)
	blk := &Block{
		Header: BlockHeader{
			ParentHash: HashType{1, 2, 4},
			ExtraData:  HashType{1, 2, 5},
		},
		Miner:     addr2,
		Timestamp: 127,
		Txs:       []*Transaction{tx},
	}
	blk.FillHash()
	err := ExecuteBlock(blk, 1000000, s)
	if err != nil {
		t.Fatal("failed to execute block")
	}
	info = GetAccountInfo(s, addr1)
	if info.Balance != 9400000 {
		t.Fatalf("account 1 balance invalid: %d", info.Balance)
	}
	info = GetAccountInfo(s, addr2)
	if info.Balance != 1600000 {
		t.Fatalf("account 2 balance invalid: %d", info.Balance)
	}
}
