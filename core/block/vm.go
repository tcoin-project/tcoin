package block

import (
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
	if call.prog > vm.MaxLoadedPrograms {
		return 0, vm.ErrIllegalPc
	}
	env := call.env
	if env.Gas < GasCall {
		return 0, vm.ErrInsufficientGas
	}
	env.Gas -= GasCall
	cpu := &ctx.cpus[call.prog]
	oldCpu := *cpu
	// todo: consider revert
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
		for {
			curPc := cpu.Pc
			if curPc == RetAddr {
				//call.s.Merge()
				return cpu.GetArg(0), nil
			}
			if (curPc >> 32) == SyscallProg {
				if (curPc & 3) != 0 {
					return 0, ErrInvalidSyscall
				}
				err = ctx.execSyscall(call, ((1<<63)-curPc)>>2)
				if err != nil {
					return 0, err
				}
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

func ExecVmTxRawCode(origin AddressType, gasLimit uint64, data []byte, s *storage.Slice, ctx *ExecutionContext) error {
	// todo: fork s outside
	const initPc = 0x10000000
	mem := &vm.Memory{}
	if gasLimit < GasVmTxRawCode {
		return vm.ErrInsufficientGas
	}
	env := &vm.ExecEnv{
		Gas: gasLimit - GasVmTxRawCode,
	}
	vmCtx := &vmCtx{
		ctx:    ctx,
		mem:    mem,
		loadId: make(map[AddressType]int),
		origin: origin,
	}
	id, _, _ := vmCtx.newProgram(origin)
	err := mem.Programs[id].LoadRawCode(data, initPc, env)
	vmCtx.entry[id] = 0
	if err != nil {
		return err
	}
	vmCtx.cpus[id].Reg[2] = DefaultSp
	_, err = vmCtx.execVM(&callCtx{
		s:         s,
		env:       env,
		pc:        initPc,
		callValue: 0,
		args:      nil,
		caller:    id,
		callType:  CallExternal,
	})
	return err
}
