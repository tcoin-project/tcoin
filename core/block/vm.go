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

var ErrInvalidJumpDest = errors.New("invalid jump dest")

type vmCtx struct {
	ctx      *ExecutionContext
	cpus     [vm.MaxLoadedPrograms]vm.CPU
	s        *storage.Slice
	mem      *vm.Memory
	loadId   map[AddressType]int
	addr     [vm.MaxLoadedPrograms]AddressType
	jumpDest map[uint64]bool
	origin   AddressType
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

func execVM(ctx *vmCtx, pc, gas, callValue uint64, args []uint64, caller int) (uint64, error) {
	const RetAddr = 0xdeadbeef00000000
	prog := pc >> 32
	if prog > vm.MaxLoadedPrograms {
		return 0, vm.ErrIllegalPc
	}
	env := &vm.ExecEnv{
		Gas: gas,
	}
	if env.Gas < GasCall {
		return 0, vm.ErrInsufficientGas
	}
	env.Gas -= GasCall
	cpu := &ctx.cpus[prog]
	oldCpu := *cpu
	oldS := ctx.s
	ctx.s = storage.ForkSlice(oldS)
	defer func() {
		*cpu = oldCpu
		ctx.s = oldS
	}()
	cpu.SetCall(pc, RetAddr)
	for i, x := range args {
		cpu.SetArg(i, x)
	}
	for {
		err := vm.Exec(cpu, ctx.mem, env)
		if err != nil {
			return 0, err
		}
		for {
			curPc := ctx.cpus[prog].Pc
			if curPc == RetAddr {
				ctx.s.Merge()
				return cpu.GetArg(0), nil
			}
			if (curPc >> 32) == SyscallProg {
				if (curPc & 3) != 0 {
					return 0, ErrInvalidSyscall
				}
				err = execSyscall(ctx, env, prog, ((1<<63)-curPc)>>2, callValue, caller)
				if err != nil {
					return 0, err
				}
				continue
			}
			if !ctx.isValidJumpDest(curPc) {
				return 0, ErrInvalidJumpDest
			}
			r, err := execVM(ctx, curPc, gas, 0, []uint64{cpu.GetArg(0), cpu.GetArg(1)}, int(prog))
			if err != nil {
				return 0, err
			}
			cpu.SetArg(0, r)
			cpu.Ret()
		}
	}
}

func ExecVmTxRawCode(origin AddressType, gasLimit uint64, data []byte, s *storage.Slice, ctx *ExecutionContext) error {
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
		s:      s,
		mem:    mem,
		loadId: make(map[AddressType]int),
		origin: origin,
	}
	id, _, _ := vmCtx.newProgram(origin)
	err := mem.Programs[id].LoadRawCode(data, initPc, env)
	if err != nil {
		return err
	}
	vmCtx.cpus[id].Reg[2] = DefaultSp
	_, err = execVM(vmCtx, initPc, env.Gas, 0, []uint64{}, id)
	return err
}
