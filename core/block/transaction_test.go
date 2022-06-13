package block

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/vm"
)

func TestTransactionSerialization(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	for tp := 1; tp <= 2; tp++ {
		tx := &Transaction{
			TxType:   byte(tp),
			Value:    rnd.Uint64(),
			GasLimit: rnd.Uint64(),
			Fee:      rnd.Uint64(),
			Nonce:    rnd.Uint64(),
			Data:     []byte{1, 2, 3},
		}
		rnd.Read(tx.SenderPubkey[:])
		rnd.Read(tx.SenderSig[:])
		if tp == 1 {
			rnd.Read(tx.Receiver[:])
		} else if tp == 2 {
			tx.Value = 0
		}

		var b bytes.Buffer
		err := EncodeTx(&b, tx)
		if err != nil {
			t.Fatal(err)
		}
		tx2, err := DecodeTx(&b)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tx, tx2) {
			t.Fatal("not equal")
		}
	}
}

func TestTransactionExecType1(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	pubk1, prik1 := GenKeyPair(rnd)
	pubk2, prik2 := GenKeyPair(rnd)
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
	err := ExecuteTx(tx, s, &ExecutionContext{})
	if err != nil {
		t.Fatal("failed to execute tx 1")
	}
	err = ExecuteTx(tx, s, &ExecutionContext{})
	if err == nil || err.Error() != "nonce mismatch" {
		t.Fatalf("expect fail, but returned %v", err)
	}
	tx = &Transaction{
		TxType:       1,
		SenderPubkey: pubk2,
		Receiver:     addr1,
		Value:        100000,
		GasLimit:     100000,
		Fee:          100000,
		Nonce:        0,
		Data:         []byte{1, 2, 3},
	}
	tx.Sign(prik2)
	err = ExecuteTx(tx, s, &ExecutionContext{})
	if err != nil {
		t.Fatal("failed to execute tx 2")
	}
	info = GetAccountInfo(s, addr1)
	if info.Balance != 9500000 {
		t.Fatalf("account 1 balance invalid: %d", info.Balance)
	}
	info = GetAccountInfo(s, addr2)
	if info.Balance != 300000 {
		t.Fatalf("account 2 balance invalid: %d", info.Balance)
	}
}

func TestTransactionExecType2(t *testing.T) {
	genCode := func(addr AddressType, value uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, addr",
			fmt.Sprintf("li a1, %d", value),
			"la a2, addr",
			"li a3, 8",
			fmt.Sprintf("li t0, -%d", SYSCALL_TRANSFER*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv ra, s0",
			"ret",
			"addr:",
			asAsmByteArr(addr[:]),
			"msg:",
			asAsmByteArr([]byte("testmsg!")),
		}, "\n")
	}
	ctx := &ExecutionContext{
		Tip1Enabled: true,
	}
	rnd := rand.New(rand.NewSource(114515))
	pubk1, prik1 := GenKeyPair(rnd)
	pubk2, prik2 := GenKeyPair(rnd)
	addr1 := PubkeyToAddress(pubk1)
	addr2 := PubkeyToAddress(pubk2)
	s := storage.EmptySlice()
	info := GetAccountInfo(s, addr1)
	info.Balance = 10000000
	SetAccountInfo(s, addr1, info)
	tx := &Transaction{
		TxType:       2,
		SenderPubkey: pubk1,
		GasLimit:     100000,
		Fee:          100000,
		Nonce:        0,
		Data:         vm.AsmToBytes(genCode(addr2, 500000)),
	}
	tx.Sign(prik1)
	err := ExecuteTx(tx, s, ctx)
	if err != nil {
		t.Fatal("failed to execute tx 1")
	}
	err = ExecuteTx(tx, s, ctx)
	if err == nil || err.Error() != "nonce mismatch" {
		t.Fatalf("expect fail, but returned %v", err)
	}
	tx = &Transaction{
		TxType:       2,
		SenderPubkey: pubk2,
		GasLimit:     100000,
		Fee:          100000,
		Nonce:        0,
		Data:         vm.AsmToBytes(genCode(addr1, 500000)),
	}
	tx.Sign(prik2)
	err = ExecuteTx(tx, s, ctx)
	if err != nil {
		t.Fatal("failed to execute tx 2")
	}
	info = GetAccountInfo(s, addr1)
	if info.Balance != 9400000 {
		t.Fatalf("account 1 balance invalid: %d", info.Balance)
	}
	info = GetAccountInfo(s, addr2)
	if info.Balance != 400000 {
		t.Fatalf("account 2 balance invalid: %d", info.Balance)
	}
}
