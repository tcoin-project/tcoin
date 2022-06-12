package block

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/vm"
)

type testContract struct {
	addr AddressType
	code string
}

type testVmCtx struct {
	t                      *testing.T
	asmCode                string
	origin                 AddressType
	gasLimit               uint64
	s                      *storage.Slice
	ecxt                   *ExecutionContext
	contracts              []testContract
	expectedError          error
	expectedGas            uint64
	expectedGasWithBaseLen uint64
}

func asAsmByteArr(s []byte) string {
	for len(s)%4 != 0 {
		s = append(s, 0)
	}
	res := []string{}
	for _, x := range s {
		res = append(res, fmt.Sprintf(".byte %d", x))
	}
	return strings.Join(res, "\n")
}

func (t *testVmCtx) run() {
	code := vm.AsmToBytes(t.asmCode)
	if t.origin == (AddressType{}) {
		t.origin = AddressType{1, 2, 3}
	}
	if t.gasLimit == 0 {
		t.gasLimit = 10000000
	}
	if t.s == nil {
		t.s = storage.EmptySlice()
	}
	for _, ct := range t.contracts {
		elf := vm.AsmToBytes(ct.code)
		storeContractCode(t.s, ct.addr, elf)
	}
	err := ExecVmTxRawCode(t.origin, t.gasLimit, code, t.s, &ExecutionContext{
		Height:     200,
		Time:       300,
		Miner:      AddressType{2, 3, 4},
		Difficulty: HashType{0, 1},
		ChainId:    345,
		Callback:   nil,
	}, nil)
	if err != t.expectedError {
		t.t.Fatalf("unexpected error: %v != %v", err, t.expectedError)
	}
}

func (t *testVmCtx) runInner() {
	const initPc = 0x10000000
	code := vm.AsmToBytes(t.asmCode)
	if t.origin == (AddressType{}) {
		t.origin = AddressType{1, 2, 3}
	}
	if t.gasLimit == 0 {
		t.gasLimit = 10000000
	}
	if t.s == nil {
		t.s = storage.EmptySlice()
	}
	if t.ecxt == nil {
		t.ecxt = &ExecutionContext{
			Height:     200,
			Time:       300,
			Miner:      AddressType{2, 3, 4},
			Difficulty: HashType{0, 1},
			ChainId:    345,
			Callback:   nil,
		}
	}
	env := &vm.ExecEnv{
		Gas: t.gasLimit,
	}
	vmCtx := newVmCtx(t.ecxt, t.origin, nil)
	id, _, _ := vmCtx.newProgram(t.origin)
	err := vmCtx.mem.Programs[id].LoadRawCode(code, initPc, env)
	vmCtx.entry[id] = 0
	if err != nil {
		t.t.Fatal(err)
	}
	totLen := len(code)
	for _, ct := range t.contracts {
		elf := vm.AsmToBytes(ct.code)
		storeContractCode(t.s, ct.addr, elf)
		tid, new, err := vmCtx.newProgram(ct.addr)
		if err != nil {
			t.t.Fatalf("failed to load contract: %v", err)
		}
		if !new {
			t.t.Fatal("address confict")
		}
		err = vmCtx.mem.Programs[tid].LoadRawCode(elf, initPc, env)
		if err != nil {
			t.t.Fatalf("failed to load contract: %v", err)
		}
		vmCtx.entry[tid] = initPc
		vmCtx.jumpDest[(uint64(tid)<<32)|initPc] = true
		vmCtx.cpus[tid].Reg[2] = (uint64(tid) << 32) | DefaultSp
		totLen += len(elf)
	}
	vmCtx.cpus[id].Reg[2] = DefaultSp
	_, err = vmCtx.execVM(&callCtx{
		s:         t.s,
		env:       env,
		pc:        initPc,
		callValue: 0,
		args:      nil,
		caller:    id,
		callType:  CallExternal,
	})
	vmCtx.mem.Recycle()
	if err != t.expectedError {
		t.t.Fatalf("unexpected error: %v != %v", err, t.expectedError)
	}
	if t.expectedGasWithBaseLen != 0 {
		t.expectedGas = t.expectedGasWithBaseLen + uint64(totLen/4)*vm.GasInstructionBase
	}
	if t.expectedGas != 0 && t.gasLimit-env.Gas != t.expectedGas {
		t.t.Fatalf("gas mismatch: %d != %d", t.gasLimit-env.Gas, t.expectedGas)
	}
	vmCtx.mem.Recycle()
}

func TestVMBasicExec(t *testing.T) {
	(&testVmCtx{
		t:             t,
		asmCode:       "ret",
		expectedError: nil,
	}).run()
	(&testVmCtx{
		t:             t,
		asmCode:       "ret",
		gasLimit:      1,
		expectedError: vm.ErrInsufficientGas,
	}).run()
	(&testVmCtx{
		t:             t,
		asmCode:       "ret",
		expectedGas:   vm.GasInstructionBase + vm.GasMemoryPage + GasCall,
		expectedError: nil,
	}).runInner()
	(&testVmCtx{
		t:       t,
		asmCode: "li a0, 3; li a1, 4; li a2, 0x110000000; mv s0, ra; jalr a2; li a1, 7; bne a0, a1, _start-2048; mv ra, s0; ret",
		contracts: []testContract{
			{addr: AddressType{6, 1}, code: "xor a0, a0, a1; ret"},
		},
		expectedGas:   vm.GasInstructionBase*12 + vm.GasMemoryPage*2 + GasCall*2,
		expectedError: nil,
	}).runInner()
}
