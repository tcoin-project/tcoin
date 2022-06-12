package block

import (
	"crypto/ed25519"
	"crypto/sha256"
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
	// todo: need to be done after load contract
	rnd := rand.New(rand.NewSource(114531))
	_ = rnd
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
}

func TestVMSyscallCreate(t *testing.T) {
	// todo (low priority)
	rnd := rand.New(rand.NewSource(114533))
	_ = rnd
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
	// todo
	rnd := rand.New(rand.NewSource(114535))
	_ = rnd
}
