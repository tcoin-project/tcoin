package block

import (
	"math/rand"
	"strings"
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

func TestVMSyscallSelf(t *testing.T) {
	s := []string{
		"mv s0, ra",
		"addi a0, sp, -32",
		"li t0, -8",
		"srli t0, t0, 1",
		"jalr t0",
		"ld a1, 0(a0)",
		"li a2, 0x807060504030201",
		"bne a1, a2, _start-2048",
		"ld a1, 8(a0)",
		"li a2, 0x102030405060708",
		"bne a1, a2, _start-2048",
		"ld a1, 16(a0)",
		"li a2, 0x401050004010100",
		"bne a1, a2, _start-2048",
		"ld a1, 24(a0)",
		"li a2, 0x1080009010901",
		"bne a1, a2, _start-2048",
		"mv ra, s0",
		"ret",
	}
	(&testVmCtx{
		t:                      t,
		asmCode:                strings.Join(s, "\n"),
		expectedGasWithBaseLen: vm.GasMemoryOp*4 + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_SELF],
		expectedError:          nil,
		origin:                 AddressType{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 1, 4, 0, 5, 1, 4, 1, 9, 1, 9, 0, 8, 1, 0},
	}).runInner()
}

func TestVMSyscallOrigin(t *testing.T) {
	s := []string{
		"mv s0, ra",
		"addi a0, sp, -32",
		"li t0, -8",
		"srli t0, t0, 1",
		"jalr t0",
		"ld a1, 0(a0)",
		"li a2, 0x807060504030201",
		"bne a1, a2, _start-2048",
		"ld a1, 8(a0)",
		"li a2, 0x102030405060708",
		"bne a1, a2, _start-2048",
		"ld a1, 16(a0)",
		"li a2, 0x401050004010100",
		"bne a1, a2, _start-2048",
		"ld a1, 24(a0)",
		"li a2, 0x1080009010901",
		"bne a1, a2, _start-2048",
		"mv ra, s0",
		"ret",
	}
	(&testVmCtx{
		t:                      t,
		asmCode:                strings.Join(s, "\n"),
		expectedGasWithBaseLen: vm.GasMemoryOp*4 + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_SELF],
		expectedError:          nil,
		origin:                 AddressType{1, 2, 3, 4, 5, 6, 7, 8, 8, 7, 6, 5, 4, 3, 2, 1, 0, 1, 1, 4, 0, 5, 1, 4, 1, 9, 1, 9, 0, 8, 1, 0},
	}).runInner()
	// todo: test subcalls
}
