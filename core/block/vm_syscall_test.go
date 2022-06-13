package block

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/vm"
	elfx "github.com/mcfx/tcoin/vm/elf"
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
	genPCallCode := func(value, gasLimit uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"li a0, 0x110000000",
			"li a1, 0",
			"li a2, 0",
			fmt.Sprintf("li a3, %d", value),
			fmt.Sprintf("li a4, %d", gasLimit),
			"addi a5, sp, -8",
			"addi a6, sp, -1200",
			fmt.Sprintf("li t0, -%d", SYSCALL_PROTECTED_CALL*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv ra, s0",
			"ret",
		}, "\n")
	}
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
	s := storage.EmptySlice()
	ai := GetAccountInfo(s, addr)
	ai.Balance = 1000000
	SetAccountInfo(s, addr, ai)
	(&testVmCtx{
		t:       t,
		s:       s,
		asmCode: genPCallCode(12345, 100000),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCmpIntCode(SYSCALL_CALLVALUE, 12345)},
		},
		expectedGasWithBaseLen: vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_CALLVALUE] + GasSyscallBase[SYSCALL_PROTECTED_CALL] + GasSyscallBase[SYSCALL_TRANSFER],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
}

func TestVMSyscallStorageStoreLoad(t *testing.T) {
	genStoreCode := func(key HashType, val storage.DataType) string {
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, key",
			"la a1, val",
			fmt.Sprintf("li t0, -%d", SYSCALL_STORAGE_STORE*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv ra, s0",
			"ret",
			"key:",
			asAsmByteArr(key[:]),
			"val:",
			asAsmByteArr(val[:]),
		}, "\n")
	}
	genLoadCode := func(key HashType, val storage.DataType) string {
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, key",
			"addi a1, sp, -32",
			fmt.Sprintf("li t0, -%d", SYSCALL_STORAGE_LOAD*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv a0, a1",
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
			"key:",
			asAsmByteArr(key[:]),
			"expected:",
			asAsmByteArr(val[:]),
		}, "\n")
	}
	rnd := rand.New(rand.NewSource(114518))
	addr := make([]AddressType, 3)
	key := make([]HashType, 2)
	val := make([]storage.DataType, len(addr)*len(key))
	for i := 0; i < len(addr); i++ {
		rnd.Read(addr[i][:])
		for j := 0; j < len(key); j++ {
			rnd.Read(val[i*len(key)+j][:])
		}
	}
	for i := 0; i < len(key); i++ {
		rnd.Read(key[i][:])
	}
	s := storage.EmptySlice()
	for i := 0; i < len(addr); i++ {
		for j := 0; j < len(key); j++ {
			(&testVmCtx{
				t:                      t,
				s:                      s,
				asmCode:                genStoreCode(key[j], val[i*len(key)+j]),
				expectedGasWithBaseLen: vm.GasInstructionBase*-16 + vm.GasMemoryPage + GasCall + GasSyscallBase[SYSCALL_STORAGE_STORE],
				expectedError:          nil,
				origin:                 addr[i],
			}).runInner()
		}
	}
	for i := 0; i < len(addr); i++ {
		for j := 0; j < len(key); j++ {
			(&testVmCtx{
				t:                      t,
				s:                      s,
				asmCode:                genLoadCode(key[j], val[i*len(key)+j]),
				expectedGasWithBaseLen: vm.GasInstructionBase*-16 + vm.GasMemoryOp*8 + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_STORAGE_LOAD],
				expectedError:          nil,
				origin:                 addr[i],
			}).runInner()
		}
	}
}

func TestVMSyscallSha256(t *testing.T) {
	genCode := func(s []byte) string {
		hash := sha256.Sum256(s)
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, str",
			fmt.Sprintf("li a1, %d", len(s)),
			"addi a2, sp, -32",
			fmt.Sprintf("li t0, -%d", SYSCALL_SHA256*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv a0, a2",
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
			"expected:",
			asAsmByteArr(hash[:]),
			"str:",
			asAsmByteArr(s),
		}, "\n")
	}
	const Len1 = 3517
	const Len2 = 142580
	rnd := rand.New(rand.NewSource(114520))
	var addr AddressType
	rnd.Read(addr[:])
	data := make([]byte, Len1)
	rnd.Read(data)
	(&testVmCtx{
		t:             t,
		asmCode:       genCode(data),
		expectedGas:   vm.GasInstructionBase*26 + vm.GasMemoryOp*8 + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_SHA256] + (Len1+63)/64*GasSyscallSha256PerBlock,
		expectedError: nil,
		origin:        addr,
	}).runInner()
	data = make([]byte, Len2)
	rnd.Read(data)
	(&testVmCtx{
		t:             t,
		asmCode:       genCode(data),
		expectedGas:   vm.GasInstructionBase*26 + vm.GasMemoryOp*8 + vm.GasMemoryPage*36 + GasCall + GasSyscallBase[SYSCALL_SHA256] + (Len2+63)/64*GasSyscallSha256PerBlock,
		expectedError: nil,
		origin:        addr,
	}).runInner()
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
	genCode := func(addr AddressType, x, y uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, addr",
			"li a1, 4096",
			fmt.Sprintf("li t0, -%d", SYSCALL_LOAD_CONTRACT*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv t0, a0",
			fmt.Sprintf("li a0, %d", x),
			fmt.Sprintf("li a1, %d", y),
			"jalr t0",
			fmt.Sprintf("li a1, %d", x+y),
			"bne a0, a1, _start-2048",
			"mv ra, s0",
			"ret",
			"addr:",
			asAsmByteArr(addr[:]),
		}, "\n")
	}
	contractCode := strings.Join([]string{
		".section .text",
		".globl _start",
		"_start:",
		"la a0, real_start",
		"mv s0, ra",
		fmt.Sprintf("li t0, -%d", SYSCALL_JUMPDEST*8),
		"srli t0, t0, 1",
		"jalr t0",
		"la a0, real_start",
		"mv ra, s0",
		"ret",
		"real_start:",
		"add a0, a0, a1",
		"ret",
	}, "\n")
	rnd := rand.New(rand.NewSource(114522))
	var addr AddressType
	rnd.Read(addr[:])
	s := storage.EmptySlice()
	elf := elfx.DebugBuildAsmELF(contractCode)
	storeContractCode(s, addr, elf)
	(&testVmCtx{
		t:                      t,
		s:                      s,
		asmCode:                genCode(addr, uint64(rnd.Int63()), uint64(rnd.Int63())),
		expectedGasWithBaseLen: vm.GasInstructionBase*(12-8) + vm.GasMemoryPage*2 + GasCall*3 + GasSyscallBase[SYSCALL_LOAD_CONTRACT] + GasSyscallBase[SYSCALL_JUMPDEST] + GasLoadContractCode + uint64(len(elf)+31)/32*GasLoadContractCodePerBlock,
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallProtectedCall(t *testing.T) {
	genCode := func(value, gasLimit, x, y uint64) string {
		s := []string{
			"mv s0, ra",
			"li a0, 0x110000000",
			fmt.Sprintf("li a1, %d", x),
			fmt.Sprintf("li a2, %d", y),
			fmt.Sprintf("li a3, %d", value),
			fmt.Sprintf("li a4, %d", gasLimit),
			"addi a5, sp, -8",
			"addi a6, sp, -1200",
			fmt.Sprintf("li t0, -%d", SYSCALL_PROTECTED_CALL*8),
			"srli t0, t0, 1",
			"jalr t0",
		}
		if gasLimit < 50+GasCall {
			s = append(s,
				"lb a0, -8(sp)",
				"bne a0, zero, _start-2048",
			)
		} else {
			s = append(s,
				"lb a1, -8(sp)",
				"beq a1, zero, _start-2048",
				fmt.Sprintf("li a1, %d", x^y),
				"bne a0, a1, _start-2048",
			)
		}
		s = append(s,
			"mv ra, s0",
			"ret",
		)
		return strings.Join(s, "\n")
	}
	rnd := rand.New(rand.NewSource(114523))
	contractCode := "xor a0, a0, a1; nop; nop; nop; ret"
	(&testVmCtx{
		t:       t,
		asmCode: genCode(0, 1000, uint64(rnd.Int63()), uint64(rnd.Int63())),
		contracts: []testContract{
			{addr: AddressType{6, 1}, code: contractCode},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-5 + vm.GasMemoryOp + vm.GasMemoryPage*3 + GasCall + GasSyscallBase[SYSCALL_PROTECTED_CALL],
		expectedError:          nil,
	}).runInner()
	(&testVmCtx{
		t:       t,
		asmCode: genCode(0, 2830, uint64(rnd.Int63()), uint64(rnd.Int63())),
		contracts: []testContract{
			{addr: AddressType{6, 1}, code: contractCode},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-2 + vm.GasMemoryOp + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_PROTECTED_CALL],
		expectedError:          nil,
	}).runInner()
	(&testVmCtx{
		t:       t,
		asmCode: genCode(0, 3500, uint64(rnd.Int63()), uint64(rnd.Int63())),
		contracts: []testContract{
			{addr: AddressType{6, 1}, code: contractCode},
		},
		expectedGasWithBaseLen: vm.GasMemoryOp + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_PROTECTED_CALL],
		expectedError:          nil,
	}).runInner()
	var addr AddressType
	var addr2 AddressType
	rnd.Read(addr[:])
	rnd.Read(addr2[:])
	s := storage.EmptySlice()
	ai := GetAccountInfo(s, addr)
	ai.Balance = 100000
	SetAccountInfo(s, addr, ai)
	(&testVmCtx{
		t:       t,
		s:       s,
		asmCode: genCode(10000, 3500, uint64(rnd.Int63()), uint64(rnd.Int63())),
		contracts: []testContract{
			{addr: addr2, code: contractCode},
		},
		expectedGasWithBaseLen: vm.GasMemoryOp + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_PROTECTED_CALL] + GasSyscallBase[SYSCALL_TRANSFER],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	ai = GetAccountInfo(s, addr)
	if ai.Balance != 90000 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	ai = GetAccountInfo(s, addr2)
	if ai.Balance != 10000 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	(&testVmCtx{
		t:       t,
		s:       s,
		asmCode: genCode(10000, 2500, uint64(rnd.Int63()), uint64(rnd.Int63())),
		contracts: []testContract{
			{addr: addr2, code: contractCode},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-5 + vm.GasMemoryOp + vm.GasMemoryPage*3 + GasCall + GasSyscallBase[SYSCALL_PROTECTED_CALL] + GasSyscallBase[SYSCALL_TRANSFER],
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	ai = GetAccountInfo(s, addr)
	if ai.Balance != 90000 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	ai = GetAccountInfo(s, addr2)
	if ai.Balance != 10000 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	(&testVmCtx{
		t:       t,
		s:       s,
		asmCode: genCode(100000, 2500, uint64(rnd.Int63()), uint64(rnd.Int63())),
		contracts: []testContract{
			{addr: addr2, code: contractCode},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-(4+5) + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_PROTECTED_CALL] + GasSyscallBase[SYSCALL_TRANSFER],
		expectedError:          ErrInsufficientBalance,
		origin:                 addr,
	}).runInner()
}

func TestVMSyscallRevert(t *testing.T) {
	genCode := func(v []byte) string {
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, str",
			fmt.Sprintf("li t0, -%d", SYSCALL_REVERT*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv ra, s0",
			"ret",
			"str:",
			asAsmByteArr(v),
		}, "\n")
	}
	genCode2 := func(gasLimit uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"li a0, 0x110000000",
			"li a3, 0",
			fmt.Sprintf("li a4, %d", gasLimit),
			"addi a5, sp, -8",
			"addi a6, sp, -1200",
			fmt.Sprintf("li t0, -%d", SYSCALL_PROTECTED_CALL*8),
			"srli t0, t0, 1",
			"jalr t0",
			"lb a0, -8(sp)",
			"bne a0, zero, _start-2048",
			"addi a0, sp, -1200",
			fmt.Sprintf("li t0, -%d", SYSCALL_REVERT*8),
			"srli t0, t0, 1",
			"jalr t0",
		}, "\n")
	}
	rnd := rand.New(rand.NewSource(114524))
	_ = rnd
	(&testVmCtx{
		t:                      t,
		asmCode:                genCode(append([]byte("testtest123456"), 0, 1)),
		expectedGasWithBaseLen: vm.GasInstructionBase*-(4+2) + vm.GasMemoryPage + GasCall + GasSyscallBase[SYSCALL_REVERT] + 14,
		expectedError:          errors.New("reverted: testtest123456"),
		origin:                 AddressType{1, 2, 3},
	}).runInner()
	(&testVmCtx{
		t:       t,
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{6, 1}, code: genCode(append([]byte("testtest123456"), 0, 1))},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-(4+2+2) + vm.GasMemoryPage*2 + GasCall*2 + GasSyscallBase[SYSCALL_REVERT] + 14,
		expectedError:          errors.New("reverted: testtest123456"),
		origin:                 AddressType{1, 2, 3},
	}).runInner()
	(&testVmCtx{
		t:       t,
		asmCode: genCode2(2820),
		contracts: []testContract{
			{addr: AddressType{6, 1}, code: genCode(append([]byte("testtest123456"), 0, 1))},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-(4+2+4) + vm.GasMemoryOp + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_REVERT] + GasSyscallBase[SYSCALL_PROTECTED_CALL] + 16,
		expectedError:          errors.New("reverted: insufficient gas"),
		origin:                 AddressType{1, 2, 3},
	}).runInner()
	(&testVmCtx{
		t:       t,
		asmCode: genCode2(100000),
		contracts: []testContract{
			{addr: AddressType{6, 1}, code: genCode(append([]byte("testtest123456"), 0, 1))},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-(4+2) + vm.GasMemoryOp + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_REVERT]*2 + GasSyscallBase[SYSCALL_PROTECTED_CALL] + 14 + 24,
		expectedError:          errors.New("reverted: reverted: testtest123456"),
		origin:                 AddressType{1, 2, 3},
	}).runInner()
}

func TestVMSyscallTime(t *testing.T) {
	rnd := rand.New(rand.NewSource(114525))
	var addr AddressType
	rnd.Read(addr[:])
	val := uint64(rnd.Int63())
	(&testVmCtx{
		t:       t,
		ectx:    &ExecutionContext{Time: val},
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
	rnd := rand.New(rand.NewSource(114526))
	var addr AddressType
	rnd.Read(addr[:])
	(&testVmCtx{
		t:       t,
		ectx:    &ExecutionContext{Miner: addr},
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCmpAddrCode(SYSCALL_MINER, addr)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_MINER],
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallBlockNumber(t *testing.T) {
	rnd := rand.New(rand.NewSource(114527))
	var addr AddressType
	rnd.Read(addr[:])
	val := uint64(rnd.Int63())
	(&testVmCtx{
		t:       t,
		ectx:    &ExecutionContext{Height: int(val)},
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
	rnd := rand.New(rand.NewSource(114528))
	var addr AddressType
	rnd.Read(addr[:])
	(&testVmCtx{
		t:       t,
		ectx:    &ExecutionContext{Difficulty: HashType(addr)},
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: AddressType{1, 2, 3}, code: genCmpAddrCode(SYSCALL_DIFFICULTY, addr)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-9 + vm.GasMemoryOp*8 + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_DIFFICULTY],
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallChainId(t *testing.T) {
	rnd := rand.New(rand.NewSource(114529))
	var addr AddressType
	rnd.Read(addr[:])
	val := uint64(rnd.Intn(10000))
	(&testVmCtx{
		t:       t,
		ectx:    &ExecutionContext{ChainId: uint16(val)},
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
	genPCallCode := func(value, gasLimit uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"li a0, 0x110000000",
			"li a1, 0",
			"li a2, 0",
			fmt.Sprintf("li a3, %d", value),
			fmt.Sprintf("li a4, %d", gasLimit),
			"addi a5, sp, -8",
			"addi a6, sp, -1200",
			fmt.Sprintf("li t0, -%d", SYSCALL_PROTECTED_CALL*8),
			"srli t0, t0, 1",
			"jalr t0",
			"lb a0, -8(sp)",
			"beq a0, zero, _start-2048",
			"mv ra, s0",
			"ret",
		}, "\n")
	}
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
	(&testVmCtx{
		t:        t,
		asmCode:  genPCallCode(0, 10000),
		gasLimit: GasLimit,
		contracts: []testContract{
			{
				addr: AddressType{1, 2, 3},
				code: genCmpIntCode(SYSCALL_GAS, 10000-vm.GasInstructionBase*4-GasCall-GasSyscallBase[SYSCALL_GAS]),
			},
		},
		expectedGasWithBaseLen: vm.GasMemoryOp + vm.GasMemoryPage*3 + GasCall*2 + GasSyscallBase[SYSCALL_GAS] + GasSyscallBase[SYSCALL_PROTECTED_CALL],
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallTransfer(t *testing.T) {
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
	rnd := rand.New(rand.NewSource(114532))
	var addr AddressType
	var addr2 AddressType
	var addr3 AddressType
	rnd.Read(addr[:])
	rnd.Read(addr2[:])
	rnd.Read(addr3[:])
	s := storage.EmptySlice()
	ai := GetAccountInfo(s, addr)
	ai.Balance = 1145140000
	SetAccountInfo(s, addr, ai)
	(&testVmCtx{
		t:                      t,
		s:                      s,
		asmCode:                genCode(addr2, 11451400),
		expectedGasWithBaseLen: vm.GasInstructionBase*-10 + vm.GasMemoryPage + GasCall + GasSyscallBase[SYSCALL_TRANSFER] + 8,
		expectedError:          nil,
		origin:                 addr,
	}).runInner()
	ai = GetAccountInfo(s, addr)
	if ai.Balance != 11451400*99 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	ai = GetAccountInfo(s, addr2)
	if ai.Balance != 114514*100 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	(&testVmCtx{
		t:       t,
		s:       s,
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: addr2, code: genCode(addr3, 114514)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-10 + vm.GasMemoryPage*2 + GasCall*2 + GasSyscallBase[SYSCALL_TRANSFER] + 8,
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
	ai = GetAccountInfo(s, addr)
	if ai.Balance != 11451400*99 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	ai = GetAccountInfo(s, addr2)
	if ai.Balance != 114514*99 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	ai = GetAccountInfo(s, addr3)
	if ai.Balance != 114514 {
		t.Fatalf("balance mismatch: %d", ai.Balance)
	}
	(&testVmCtx{
		t:       t,
		s:       s,
		asmCode: genSimpleCallCode(1),
		contracts: []testContract{
			{addr: addr2, code: genCode(addr3, 11451400)},
		},
		expectedGasWithBaseLen: vm.GasInstructionBase*-14 + vm.GasMemoryPage*2 + GasCall*2 + GasSyscallBase[SYSCALL_TRANSFER],
		expectedError:          ErrInsufficientBalance,
		origin:                 AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallCreate(t *testing.T) {
	genCreateCode := func(elf, addr []byte, flags, nonce uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"j p2",
			"expected:",
			asAsmByteArr(addr),
			"p2:",
			"addi a0, sp, -32",
			"la a1, code",
			fmt.Sprintf("li a2, %d", len(elf)),
			fmt.Sprintf("li a3, %d", flags),
			fmt.Sprintf("li a4, %d", nonce),
			fmt.Sprintf("li t0, -%d", SYSCALL_CREATE*8),
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
			"code:",
			asAsmByteArr(elf),
			asAsmByteArr([]byte{1, 2, 3, 4}),
		}, "\n")
	}
	genTestCode := func(addr []byte, x, y uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"j p2",
			"addr:",
			asAsmByteArr(addr),
			"p2:",
			"la a0, addr",
			fmt.Sprintf("li t0, -%d", SYSCALL_LOAD_CONTRACT*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv t0, a0",
			fmt.Sprintf("li a0, %d", x),
			fmt.Sprintf("li a1, %d", y),
			"jalr t0",
			fmt.Sprintf("li a1, %d", x^y),
			"bne a0, a1, _start-2048",
			"mv ra, s0",
			"ret",
		}, "\n")
	}
	getAddr := func(addr AddressType, flags, nonce uint64, elf []byte) []byte {
		s := make([]byte, AddressLen+16)
		copy(s[:AddressLen], addr[:])
		binary.LittleEndian.PutUint64(s[AddressLen:AddressLen+8], flags)
		binary.LittleEndian.PutUint64(s[AddressLen+8:AddressLen+16], nonce)
		s = append(s, elf...)
		t := sha256.Sum256(s)
		return t[:]
	}
	contractCode := strings.Join([]string{
		".section .text",
		".globl _start",
		"_start:",
		"la a0, real_start",
		"mv s0, ra",
		fmt.Sprintf("li t0, -%d", SYSCALL_JUMPDEST*8),
		"srli t0, t0, 1",
		"jalr t0",
		"la a0, real_start",
		"mv ra, s0",
		"ret",
		"real_start:",
		"xor a0, a0, a1",
		"ret",
	}, "\n")
	contractCode2 := "long start3(long a, long b) { return a^b; }" +
		fmt.Sprintf("void *start2() { void (*call)(void*) = (-%dull)>>1; call(start3); return start3; }", SYSCALL_JUMPDEST*8) +
		"__attribute__((section(\".init_code\"))) void *_start() { return start2; }"
	elf := elfx.DebugBuildAsmELF(contractCode)
	elf2 := elfx.DebugBuildELF(contractCode2)
	e, err := elfx.ParseELF(elf)
	if err != nil {
		t.Fatalf("error happened: %v", err)
	}
	elfs, err := elfx.TrimELF(elf, e, nil, uint64(e.Entry))
	if err != nil {
		t.Fatalf("error happened: %v", err)
	}
	rnd := rand.New(rand.NewSource(114533))
	var addr AddressType
	rnd.Read(addr[:])
	s := storage.EmptySlice()
	addrs := [][]byte{}
	for i := 0; i < 5; i++ {
		x := uint64(i)
		var y uint64 = 0
		var f uint64 = 0
		if i >= 3 {
			x = uint64(rnd.Int63())
			y = x
			f = CREATE_USENONCE
		}
		addrs = append(addrs, getAddr(addr, f, x, elf))
		(&testVmCtx{
			t:       t,
			s:       s,
			asmCode: genCreateCode(elf, addrs[len(addrs)-1], f, y),
			expectedGasWithBaseLen: vm.GasInstructionBase*-(9+uint64(len(elf))/4) +
				vm.GasMemoryOp*8 +
				vm.GasMemoryPage*2 +
				GasCall +
				GasSyscallBase[SYSCALL_CREATE] +
				uint64(len(elf)) +
				(uint64(len(elf))+31)/32*GasSyscallCreateStorePerBlock,
			expectedError: nil,
			origin:        addr,
		}).runInner()
	}
	addrs = append(addrs, getAddr(addr, CREATE_TRIMELF, 3, elf))
	(&testVmCtx{
		t:       t,
		s:       s,
		asmCode: genCreateCode(elf, addrs[len(addrs)-1], CREATE_TRIMELF, 0),
		expectedGasWithBaseLen: vm.GasInstructionBase*-(9+uint64(len(elf))/4) +
			vm.GasMemoryOp*8 +
			vm.GasMemoryPage*2 +
			GasCall +
			GasSyscallBase[SYSCALL_CREATE] +
			uint64(len(elf)) +
			(uint64(len(elfs))+31)/32*GasSyscallCreateStorePerBlock,
		expectedError: nil,
		origin:        addr,
	}).runInner()
	(&testVmCtx{
		t:             t,
		s:             s,
		asmCode:       genCreateCode(elf2, getAddr(addr, CREATE_INIT, 4, elf2), CREATE_INIT, 0),
		expectedError: nil,
		origin:        addr,
	}).runInner()
	addrs = append(addrs, getAddr(addr, CREATE_INIT|CREATE_TRIMELF, 5, elf2))
	(&testVmCtx{
		t:             t,
		s:             s,
		asmCode:       genCreateCode(elf2, addrs[len(addrs)-1], CREATE_INIT|CREATE_TRIMELF, 0),
		expectedError: nil,
		origin:        addr,
	}).runInner()
	for _, x := range addrs {
		(&testVmCtx{
			t:             t,
			s:             s,
			asmCode:       genTestCode(x, uint64(rand.Int63()), uint64(rand.Int63())),
			expectedError: nil,
			origin:        addr,
		}).runInner()
	}
}

func TestVMSyscallEd25519Verify(t *testing.T) {
	genCode := func(msg, pubkey, sig []byte, expected uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, msg",
			fmt.Sprintf("li a1, %d", len(msg)),
			"la a2, pubkey",
			"la a3, sig",
			fmt.Sprintf("li t0, -%d", SYSCALL_ED25519_VERIFY*8),
			"srli t0, t0, 1",
			"jalr t0",
			fmt.Sprintf("li a1, %d", expected),
			"bne a0, a1, _start-2048",
			"mv ra, s0",
			"ret",
			"pubkey:",
			asAsmByteArr(pubkey),
			"sig:",
			asAsmByteArr(sig),
			"msg:",
			asAsmByteArr(msg),
		}, "\n")
	}
	const Len = 1535
	rnd := rand.New(rand.NewSource(114534))
	pubkey, privkey, _ := ed25519.GenerateKey(rnd)
	msg := make([]byte, Len)
	rnd.Read(msg)
	sig := ed25519.Sign(privkey, msg)
	(&testVmCtx{
		t:             t,
		asmCode:       genCode(msg, pubkey, sig, 1),
		expectedGas:   vm.GasInstructionBase*15 + vm.GasMemoryPage + GasCall + GasSyscallBase[SYSCALL_ED25519_VERIFY] + (Len+127)/128*GasSyscallEd25519PerBlock,
		expectedError: nil,
		origin:        AddressType{4, 5, 6},
	}).runInner()
	sig[3] = 5
	(&testVmCtx{
		t:             t,
		asmCode:       genCode(msg, pubkey, sig, 0),
		expectedGas:   vm.GasInstructionBase*15 + vm.GasMemoryPage + GasCall + GasSyscallBase[SYSCALL_ED25519_VERIFY] + (Len+127)/128*GasSyscallEd25519PerBlock,
		expectedError: nil,
		origin:        AddressType{4, 5, 6},
	}).runInner()
}

func TestVMSyscallLoadELF(t *testing.T) {
	genCode := func(addr AddressType, x, y uint64) string {
		return strings.Join([]string{
			"mv s0, ra",
			"la a0, addr",
			"li a1, 4096",
			fmt.Sprintf("li t0, -%d", SYSCALL_LOAD_ELF*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv t0, a0",
			fmt.Sprintf("li a0, %d", x),
			fmt.Sprintf("li a1, %d", y),
			"jalr t0",
			fmt.Sprintf("li a1, %d", x^y),
			"bne a0, a1, _start-2048",
			"mv ra, s0",
			"ret",
			"addr:",
			asAsmByteArr(addr[:]),
		}, "\n")
	}
	contractCode := "__attribute__((optimize(2))) long _start(long a, long b) { return a ^ b; }"
	rnd := rand.New(rand.NewSource(114535))
	var addr AddressType
	rnd.Read(addr[:])
	s := storage.EmptySlice()
	elf := elfx.DebugBuildELF(contractCode)
	storeContractCode(s, addr, elf)
	(&testVmCtx{
		t:                      t,
		s:                      s,
		asmCode:                genCode(addr, uint64(rnd.Int63()), uint64(rnd.Int63())),
		expectedGasWithBaseLen: vm.GasInstructionBase*(2-8) + vm.GasMemoryPage*2 + GasCall + GasSyscallBase[SYSCALL_LOAD_ELF] + GasLoadContractCode + uint64(len(elf)+31)/32*GasLoadContractCodePerBlock,
		expectedError:          nil,
		origin:                 AddressType{4, 5, 6},
		callType:               CallInit,
	}).runInner()
	genCode2 := func(addr AddressType, x, y uint64) string {
		return strings.Join([]string{
			"mv s1, ra",
			"la a0, addr",
			"li a1, 4096",
			fmt.Sprintf("li t0, -%d", SYSCALL_LOAD_CONTRACT*8),
			"srli t0, t0, 1",
			"jalr t0",
			"mv t0, a0",
			fmt.Sprintf("li a0, %d", x),
			fmt.Sprintf("li a1, %d", y),
			"jalr t0",
			fmt.Sprintf("li a1, %d", x-y),
			"bne a0, a1, _start-2048",
			"la a0, addr",
			"li a1, 4096",
			fmt.Sprintf("li t0, -%d", SYSCALL_LOAD_ELF*8),
			"srli t0, t0, 1",
			"jalr t0",
			"jalr a0",
			"mv t0, a0",
			fmt.Sprintf("li a0, %d", x),
			fmt.Sprintf("li a1, %d", y),
			"jalr t0",
			fmt.Sprintf("li a1, %d", x-y),
			"bne a0, a1, _start-2048",
			"mv ra, s1",
			"ret",
			"addr:",
			asAsmByteArr(addr[:]),
		}, "\n")
	}
	contractCode2 := strings.Join([]string{
		".section .text",
		".globl _start",
		"_start:",
		"la a0, real_start",
		"mv s0, ra",
		fmt.Sprintf("li t0, -%d", SYSCALL_JUMPDEST*8),
		"srli t0, t0, 1",
		"jalr t0",
		"la a0, real_start",
		"mv ra, s0",
		"ret",
		"real_start:",
		"sub a0, a0, a1",
		"ret",
	}, "\n")
	rnd.Read(addr[:])
	s = storage.EmptySlice()
	elf = elfx.DebugBuildAsmELF(contractCode2)
	storeContractCode(s, addr, elf)
	(&testVmCtx{
		t:                      t,
		s:                      s,
		asmCode:                genCode2(addr, uint64(rnd.Int63()), uint64(rnd.Int63())),
		expectedGasWithBaseLen: vm.GasInstructionBase*(12*2-8) + vm.GasMemoryPage*3 + GasCall*3 + GasSyscallBase[SYSCALL_LOAD_CONTRACT] + GasSyscallBase[SYSCALL_LOAD_ELF] + GasSyscallBase[SYSCALL_JUMPDEST]*2 + GasLoadContractCodeCached + GasLoadContractCode + uint64(len(elf)+31)/32*GasLoadContractCodePerBlock,
		expectedError:          nil,
		origin:                 AddressType{1, 2, 3},
		callType:               CallInit,
	}).runInner()
}
