package block

import (
	"encoding/binary"
	"errors"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/vm"
)

const DefaultSp = 0x30000000 + vm.PageSize*vm.MaxPagesPerBlock - 16
const SyscallProg = 0x7fffffff
const GasCall = 2800
const GasVmTxRawCode = 5000

const CallExternal = 1
const CallStart = 2
const CallRegular = 3
const CallInit = 4
const CallView = 5

var ErrInvalidJumpDest = errors.New("invalid jump dest")

type vmCtx struct {
	ctx      *ExecutionContext
	cpus     [vm.MaxLoadedPrograms]vm.CPU
	mem      *vm.Memory
	loadId   map[AddressType]int
	elfCache map[AddressType][]byte
	addr     [vm.MaxLoadedPrograms]AddressType
	entry    [vm.MaxLoadedPrograms]uint32
	jumpDest map[uint64]bool
	origin   AddressType
	tx       *Transaction
}

type callCtx struct {
	s         *storage.Slice
	env       *vm.ExecEnv
	pc        uint64
	prog      uint64
	callValue uint64
	args      []uint64
	caller    int
	callType  int
}

func newVmCtx(ctx *ExecutionContext, origin AddressType, tx *Transaction) *vmCtx {
	return &vmCtx{
		ctx:      ctx,
		mem:      &vm.Memory{},
		loadId:   make(map[AddressType]int),
		elfCache: make(map[AddressType][]byte),
		jumpDest: make(map[uint64]bool),
		origin:   origin,
		tx:       tx,
	}
}

func (ctx *vmCtx) newProgram(addr AddressType) (int, bool, error) {
	if x, ok := ctx.loadId[addr]; ok {
		return x, false, nil
	}
	id, err := ctx.mem.NewProgram()
	if err != nil {
		return 0, false, err
	}
	ctx.addr[id] = addr
	ctx.loadId[addr] = id
	return id, true, nil
}

func (ctx *vmCtx) isValidJumpDest(addr uint64) bool {
	_, ok := ctx.jumpDest[addr]
	return ok
}

func (ctx *vmCtx) execVM(call *callCtx) (uint64, error) {
	const RetAddr = 0xdeadbeef00000000
	call.prog = call.pc >> 32
	if call.prog >= vm.MaxLoadedPrograms {
		return 0, vm.ErrIllegalPc
	}
	env := call.env
	if env.Gas < GasCall {
		return 0, vm.ErrInsufficientGas
	}
	env.Gas -= GasCall
	cpu := &ctx.cpus[call.prog]
	oldCpu := *cpu
	defer func() {
		*cpu = oldCpu
	}()
	cpu.SetCall(call.pc, RetAddr)
	for i, x := range call.args {
		cpu.SetArg(i, x)
	}
	for {
		err := vm.Exec(cpu, ctx.mem, env)
		if err != nil {
			return 0, err
		}
		for (cpu.Pc >> 32) != call.prog {
			curPc := cpu.Pc
			if curPc == RetAddr {
				//fmt.Printf("gas: %d\n", env.Gas)
				return cpu.GetArg(0), nil
			}
			if (curPc >> 32) == SyscallProg {
				//fmt.Printf("syscall: %x %d\n", curPc, ((1<<63)-curPc)>>2)
				if (curPc & 3) != 0 {
					return 0, ErrInvalidSyscall
				}
				//fmt.Printf("gas1: %d\n", env.Gas)
				err = ctx.execSyscall(call, ((1<<63)-curPc)>>2)
				//fmt.Printf("gas2: %d\n", env.Gas)
				if err != nil {
					return 0, err
				}
				cpu.Ret()
				continue
			}
			if !ctx.isValidJumpDest(curPc) {
				return 0, ErrInvalidJumpDest
			}
			r, err := ctx.execVM(&callCtx{
				s:         call.s,
				env:       env,
				pc:        curPc,
				callValue: 0,
				args:      []uint64{cpu.GetArg(0), cpu.GetArg(1)},
				caller:    int(call.prog),
				callType:  CallRegular,
			})
			if err != nil {
				return 0, err
			}
			cpu.SetArg(0, r)
			cpu.Ret()
		}
	}
}

func ExecVmTxRawCode(origin AddressType, gasLimit uint64, data []byte, s *storage.Slice, ctx *ExecutionContext, tx *Transaction) (uint64, error) {
	const initPc = 0x10000000
	if gasLimit < GasVmTxRawCode {
		return gasLimit, vm.ErrInsufficientGas
	}
	env := &vm.ExecEnv{
		Gas: gasLimit - GasVmTxRawCode,
	}
	vmCtx := newVmCtx(ctx, origin, tx)
	defer vmCtx.mem.Recycle()
	id, _, _ := vmCtx.newProgram(origin)
	err := vmCtx.mem.Programs[id].LoadRawCode(data, initPc, env)
	vmCtx.entry[id] = 0
	if err != nil {
		return env.Gas, err
	}
	vmCtx.cpus[id].Reg[2] = (uint64(id) << 32) | DefaultSp
	_, err = vmCtx.execVM(&callCtx{
		s:         s,
		env:       env,
		pc:        initPc,
		callValue: 0,
		args:      nil,
		caller:    id,
		callType:  CallExternal,
	})
	return env.Gas, err
}

func ExecVmViewRawCode(origin AddressType, gasLimit uint64, data []byte, s *storage.Slice, ctx *ExecutionContext) ([]byte, error) {
	const initPc = 0x10000000
	env := &vm.ExecEnv{
		Gas: gasLimit,
	}
	vmCtx := newVmCtx(ctx, origin, nil)
	defer vmCtx.mem.Recycle()
	id, _, _ := vmCtx.newProgram(origin)
	err := vmCtx.mem.Programs[id].LoadRawCode(data, initPc, env)
	vmCtx.entry[id] = 0
	if err != nil {
		return nil, err
	}
	vmCtx.cpus[id].Reg[2] = (uint64(id) << 32) | DefaultSp
	ret, err := vmCtx.execVM(&callCtx{
		s:         s,
		env:       env,
		pc:        initPc,
		callValue: 0,
		args:      nil,
		caller:    id,
		callType:  CallView,
	})
	if err != nil {
		return nil, err
	}
	lenb := make([]byte, 8)
	err = vmCtx.mem.ReadBytes(0, ret, lenb, env)
	if err != nil {
		return nil, err
	}
	n := binary.LittleEndian.Uint64(lenb)
	if n > (1 << 20) {
		return nil, err
	}
	if env.Gas < n {
		return nil, vm.ErrInsufficientGas
	}
	res := make([]byte, n)
	err = vmCtx.mem.ReadBytes(0, ret+8, res, env)
	return res, err
}
