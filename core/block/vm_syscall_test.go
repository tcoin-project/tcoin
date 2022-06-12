package block

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/vm"
)

func genSimpleCallCode(prog int) string {
	return fmt.Sprintf("li a2, %d; slli a2, a2, 28; mv s0, ra; jalr a2; mv ra, s0; ret", (prog<<4)|1)
}

func genCmpAddrCode(syscallId int, addr AddressType) string {
	s := []string{
		"mv s0, ra",
		"addi a0, sp, -32",
		fmt.Sprintf("li t0, -%d", syscallId*8),
		"srli t0, t0, 1",
		"jalr t0",
		"la a3, expected",
		"ld a1, 0(a0)",
		"ld a2, 0(a3)",
		"bne a1, a2, _start-2048",
		"ld a1, 8(a0)",
		"ld a2, 8(a3)",
		"bne a1, a2, _start-2048",
		"ld a1, 16(a0)",
		"ld a2, 16(a3)",
		"bne a1, a2, _start-2048",
		"ld a1, 24(a0)",
		"ld a2, 24(a3)",
		"bne a1, a2, _start-2048",
		"mv ra, s0",
		"ret",
		"nop",
		"expected:",
		asAsmByteArr(addr[:]),
	}
	return strings.Join(s, "\n")
}

func genCmpIntCode(syscallId int, target uint64) string {
	s := []string{
		"mv s0, ra",
		fmt.Sprintf("li t0, -%d", syscallId*8),
		"srli t0, t0, 1",
		"jalr t0",
		fmt.Sprintf("li a1, %d", target),
		"bne a0, a1, _start-2048",
		"mv ra, s0",
		"ret",
	}
	return strings.Join(s, "\n")
}

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
	}, AddressType{1, 2, 3}, nil)
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
	rnd := rand.New(rand.NewSource(114514))
	var addr AddressType
	rnd.Read(addr[:])
	(&testVmCtx{
		t:                      t,
		asmCode:                genCmpAddrCode(SYSCALL_SELF, addr),
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_SELF],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
}

func TestVMSyscallOrigin(t *testing.T) {
	rnd := rand.New(rand.NewSource(114515))
	var addr AddressType
	rnd.Read(addr[:])
	(&testVmCtx{
		t:                      t,
		asmCode:                genCmpAddrCode(SYSCALL_ORIGIN, addr),
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_ORIGIN],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	rnd.Read(addr[:])
	(&testVmCtx{
		t:       t,
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCmpAddrCode(SYSCALL_ORIGIN, addr)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_ORIGIN],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	rnd.Read(addr[:])
	(&testVmCtx{
		t:       t,
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genSimpleCallCode(2)},
			{addr: AddressType{4, 5, 6}, code: genCmpAddrCode(SYSCALL_ORIGIN, addr)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*4 + GasCall*3 + GasSyscallBase[SYSCALL_ORIGIN],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
}

func TestVMSyscallCaller(t *testing.T) {
	rnd := rand.New(rand.NewSource(114516))
	var addr AddressType
	rnd.Read(addr[:])
	(&testVmCtx{
		t:                      t,
		asmCode:                genCmpAddrCode(SYSCALL_CALLER, addr),
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_CALLER],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	rnd.Read(addr[:])
	(&testVmCtx{
		t:       t,
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCmpAddrCode(SYSCALL_CALLER, addr)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_CALLER],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	rnd.Read(addr[:])
	(&testVmCtx{
		t:       t,
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: addr, code: genSimpleCallCode(2)},
			{addr: AddressType{4, 5, 6}, code: genCmpAddrCode(SYSCALL_CALLER, addr)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*4 + GasCall*3 + GasSyscallBase[SYSCALL_CALLER],
		expectedError:          nil,
		origin:                 AddressType{1, 2, 3},
	}).runInner()
}

func TestVMSyscallCallValue(t *testing.T) {
	rnd := rand.New(rand.NewSource(114517))
	var addr AddressType
	rnd.Read(addr[:])
	(&testVmCtx{
		t:                      t,
		asmCode:                genCmpIntCode(SYSCALL_CALLVALUE, 0),
		expectedGasWithBaseLen: vm.GasMemoryPage + GasCall + GasSyscallBase[SYSCALL_CALLVALUE],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	// todo: after test protected call
}

func TestVMSyscallStorageStore(t *testing.T) {
	// todo
	rnd := rand.New(rand.NewSource(114518))
	_ = rnd
}

func TestVMSyscallStorageLoad(t *testing.T) {
	// todo
	rnd := rand.New(rand.NewSource(114519))
	_ = rnd
}

func TestVMSyscallSha256(t *testing.T) {
	// todo
	rnd := rand.New(rand.NewSource(114520))
	_ = rnd
}

func TestVMSyscallBalance(t *testing.T) {
	genCode := func(addr AddressType, expected uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, addr",
			fmt.Sprintf("li t0, -%d", SYSCALL_BALANCE*8),
			"srli t0, t0, 1",
			"jalr t0",
			fmt.Sprintf("li a1, %d", expected),
			"bne a0, a1, _start-2048",
			"mv ra, s0",
			"ret",
			"addr:",
			asAsmByteArr(addr[:]),
		}, "\n")
	}
	rnd := rand.New(rand.NewSource(114521))
	var addr AddressType
	rnd.Read(addr[:])
	s := storage.EmptySlice()
	ai := GetAccountInfo(s, addr)
	ai.Balance = 114514
	SetAccountInfo(s, addr, ai)
	(&testVmCtx{
		t:                      t,
		s:                      s,
		asmCode:                genCode(addr, 114514),
		expectedGasWithBaseLen: vm.GasInstructionBase*-8 + vm.GasMemoryPage + GasCall + GasSyscallBase[SYSCALL_BALANCE],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	(&testVmCtx{
		t:       t,
		s:       s,
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCode(addr, 114514)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-8 + vm.GasMemoryPage*2 + GasCall*2 + GasSyscallBase[SYSCALL_BALANCE],
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallLoadContract(t *testing.T) {
	// todo (low priority)
	rnd := rand.New(rand.NewSource(114522))
	_ = rnd
}

func TestVMSyscallProtectedCall(t *testing.T) {
	// todo (low priority)
	rnd := rand.New(rand.NewSource(114523))
	_ = rnd
}

func TestVMSyscallRevert(t *testing.T) {
	// todo (after protected call)
	rnd := rand.New(rand.NewSource(114524))
	_ = rnd
}

func TestVMSyscallTime(t *testing.T) {
	rnd := rand.New(rand.NewSource(114525))
	var addr AddressType
	rnd.Read(addr[:])
	val := uint64(rnd.Int63())
	(&testVmCtx{
		t:       t,
		ecxt:    &ExecutionContext{Time: val},
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCmpIntCode(SYSCALL_TIME, val)},
		},
		expectedGasWithBaseLen: vm.GasMemoryPage*2 + GasCall*2 + GasSyscallBase[SYSCALL_TIME],
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallMiner(t *testing.T) {
	// todo (soon)
	rnd := rand.New(rand.NewSource(114526))
	_ = rnd
}

func TestVMSyscallBlockNumber(t *testing.T) {
	rnd := rand.New(rand.NewSource(114527))
	var addr AddressType
	rnd.Read(addr[:])
	val := uint64(rnd.Int63())
	(&testVmCtx{
		t:       t,
		ecxt:    &ExecutionContext{Height: int(val)},
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCmpIntCode(SYSCALL_BLOCK_NUMBER, val)},
		},
		expectedGasWithBaseLen: vm.GasMemoryPage*2 + GasCall*2 + GasSyscallBase[SYSCALL_BLOCK_NUMBER],
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallDifficulty(t *testing.T) {
	// todo (soon)
	rnd := rand.New(rand.NewSource(114528))
	_ = rnd
}

func TestVMSyscallChainId(t *testing.T) {
	rnd := rand.New(rand.NewSource(114529))
	var addr AddressType
	rnd.Read(addr[:])
	val := uint64(rnd.Intn(10000))
	(&testVmCtx{
		t:       t,
		ecxt:    &ExecutionContext{ChainId: uint16(val)},
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCmpIntCode(SYSCALL_CHAINID, val)},
		},
		expectedGasWithBaseLen: vm.GasMemoryPage*2 + GasCall*2 + GasSyscallBase[SYSCALL_CHAINID],
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallGas(t *testing.T) {
	const GasLimit = 1000000
	rnd := rand.New(rand.NewSource(114530))
	var addr AddressType
	rnd.Read(addr[:])
	(&testVmCtx{
		t:        t,
		asmCode:  genSimpleCallCode(1),
		gasLimit: GasLimit,
		contracts: []testContract{
			{
				addr: AddressType{1, 2, 3},
				code: genCmpIntCode(SYSCALL_GAS, GasLimit-vm.GasInstructionBase*8-vm.GasMemoryPage*2-GasCall*2-GasSyscallBase[SYSCALL_GAS]),
			},
		},
		expectedGasWithBaseLen: vm.GasMemoryPage*2 + GasCall*2 + GasSyscallBase[SYSCALL_GAS],
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
	// todo: test protected call?
}

func TestVMSyscallJumpDest(t *testing.T) {
	// todo
	rnd := rand.New(rand.NewSource(114531))
	_ = rnd
}

func TestVMSyscallTransfer(t *testing.T) {
	// todo
	rnd := rand.New(rand.NewSource(114532))
	_ = rnd
}

func TestVMSyscallCreate(t *testing.T) {
	// todo (low priority)
	rnd := rand.New(rand.NewSource(114533))
	_ = rnd
}

func TestVMSyscallEd25519Verify(t *testing.T) {
	// todo
	rnd := rand.New(rand.NewSource(114534))
	_ = rnd
}

func TestVMSyscallLoadELF(t *testing.T) {
	// todo
	rnd := rand.New(rand.NewSource(114535))
	_ = rnd
}
