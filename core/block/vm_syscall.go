package block

import (
	"errors"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/vm"
)

const SYSCALL_SELF = 1
const SYSCALL_ORIGIN = 2
const SYSCALL_CALLER = 3
const SYSCALL_CALLVALUE = 4
const SYSCALL_STORAGE_STORE = 5
const SYSCALL_STORAGE_LOAD = 6
const SYSCALL_SHA256 = 7
const SYSCALL_BALANCE = 8
const SYSCALL_LOAD_CONTRACT = 9
const SYSCALL_PROTECTED_CALL = 10
const SYSCALL_REVERT = 11
const SYSCALL_TIME = 12
const SYSCALL_MINER = 13
const SYSCALL_BLOCK_NUMBER = 14
const SYSCALL_DIFFICULTY = 15
const SYSCALL_CHAINID = 16
const SYSCALL_GAS = 17
const SYSCALL_JUMPDEST = 18

var ErrInvalidSyscall = errors.New("invalid syscall")

var GasSyscallBase = map[int]uint64{
	SYSCALL_SELF:           40,
	SYSCALL_ORIGIN:         40,
	SYSCALL_CALLER:         40,
	SYSCALL_CALLVALUE:      40,
	SYSCALL_STORAGE_STORE:  50000,
	SYSCALL_STORAGE_LOAD:   50000,
	SYSCALL_SHA256:         200,
	SYSCALL_BALANCE:        20000,
	SYSCALL_LOAD_CONTRACT:  20000,
	SYSCALL_PROTECTED_CALL: GasCall + 1000,
	SYSCALL_REVERT:         80,
	SYSCALL_TIME:           40,
	SYSCALL_MINER:          40,
	SYSCALL_BLOCK_NUMBER:   40,
	SYSCALL_DIFFICULTY:     40,
	SYSCALL_CHAINID:        40,
	SYSCALL_GAS:            40,
	SYSCALL_JUMPDEST:       200,
}

const GasSyscallSha256PerBlock = 30
const GasSyscallLoadContractPerBlock = 3000

func execSyscall(ctx *vmCtx, env *vm.ExecEnv, prog, syscallId, callValue uint64, caller int) error {
	cpu := &ctx.cpus[prog]
	mem := ctx.mem
	switch syscallId {
	case SYSCALL_SELF:
		err := mem.WriteBytes(prog, cpu.GetArg(0), ctx.addr[prog][:], env)
		if err != nil {
			return err
		}
	case SYSCALL_ORIGIN:
		err := mem.WriteBytes(prog, cpu.GetArg(0), ctx.origin[:], env)
		if err != nil {
			return err
		}
	case SYSCALL_CALLER:
		err := mem.WriteBytes(prog, cpu.GetArg(0), ctx.addr[caller][:], env)
		if err != nil {
			return err
		}
	case SYSCALL_CALLVALUE:
		cpu.SetArg(0, callValue)
	case SYSCALL_STORAGE_STORE:
		a, err := mem.ReadBytes(prog, cpu.GetArg(0), HashLen, env)
		if err != nil {
			return err
		}
		b, err := mem.ReadBytes(prog, cpu.GetArg(1), HashLen, env)
		if err != nil {
			return err
		}
		key := storage.KeyType{}
		val := storage.DataType{}
		key[0] = 2
		copy(key[1:33], ctx.addr[prog][:])
		copy(key[33:65], a)
		copy(val[:], b)
		ctx.s.Write(key, val)
	case SYSCALL_STORAGE_LOAD:
		a, err := mem.ReadBytes(prog, cpu.GetArg(0), HashLen, env)
		if err != nil {
			return err
		}
		key := storage.KeyType{}
		key[0] = 2
		copy(key[1:33], ctx.addr[prog][:])
		copy(key[33:65], a)
		val := ctx.s.Read(key)
		err = mem.WriteBytes(prog, cpu.GetArg(1), val[:], env)
		if err != nil {
			return err
		}
	case SYSCALL_SHA256:
		//
	}
	return nil
}
