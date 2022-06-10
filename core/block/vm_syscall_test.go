package block

import (
	"math/rand"
	"testing"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/vm"
)

func TestContractLoadStore(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	s := storage.EmptySlice()
	ctx := newVmCtx(&ExecutionContext{
		Height:     200,
		Time:       300,
		Miner:      AddressType{2, 3, 4},
		Difficulty: HashType{0, 1},
		ChainId:    345,
		Callback:   nil,
	}, AddressType{1, 2, 3})
	call := &callCtx{
		s:         s,
		env:       &vm.ExecEnv{Gas: 100000000},
		prog:      0,
		callValue: 0,
		args:      nil,
		caller:    0,
		callType:  CallRegular,
	}
	ctx.newProgram(ctx.origin)
	cAddr := AddressType{3, 4, 5}
	elf := make([]byte, 135713)
	rnd.Read(elf)
	storeContractCode(s, cAddr, elf)
	r, err := ctx.loadContractCode(call, cAddr)
	if err != nil {
		t.Fatalf("error happened: %v", err)
	}
	if len(elf) != len(r) {
		t.Fatalf("stored elf length not equal: %d != %d", len(elf), len(r))
	}
	for i := 0; i < len(elf); i++ {
		if elf[i] != r[i] {
			t.Fatalf("stored elf not equal at %d: %d != %d", i, elf[i], r[i])
		}
	}
}
