package block

import (
	"crypto/sha256"
	"errors"
	"fmt"

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
var ErrIllegalSyscallParameters = errors.New("illegal syscall parameters")

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
	SYSCALL_REVERT:         500,
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
const GasSyscallRevertPerByte = 2

func execSyscall(ctx *vmCtx, env *vm.ExecEnv, prog, syscallId, callValue uint64, caller int) error {
	cpu := &ctx.cpus[prog]
	mem := ctx.mem
	if gasBase, ok := GasSyscallBase[int(syscallId)]; ok {
		if env.Gas < gasBase {
			return vm.ErrInsufficientGas
		}
		env.Gas -= gasBase
	} else {
		return ErrInvalidSyscall
	}
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
		key := storage.KeyType{}
		val := storage.DataType{}
		key[0] = 2
		copy(key[1:33], ctx.addr[prog][:])
		err := mem.ReadBytes(prog, cpu.GetArg(0), key[1:33], env)
		if err != nil {
			return err
		}
		err = mem.ReadBytes(prog, cpu.GetArg(1), val[:], env)
		if err != nil {
			return err
		}
		ctx.s.Write(key, val)
	case SYSCALL_STORAGE_LOAD:
		key := storage.KeyType{}
		key[0] = 2
		copy(key[1:33], ctx.addr[prog][:])
		err := mem.ReadBytes(prog, cpu.GetArg(0), key[1:33], env)
		if err != nil {
			return err
		}
		val := ctx.s.Read(key)
		err = mem.WriteBytes(prog, cpu.GetArg(1), val[:], env)
		if err != nil {
			return err
		}
	case SYSCALL_SHA256:
		n := cpu.GetArg(1)
		if n > (1 << 32) {
			return ErrIllegalSyscallParameters
		}
		nBlocks := (n + 63) / 64
		gas := nBlocks * GasSyscallSha256PerBlock
		if env.Gas < gas {
			return vm.ErrInsufficientGas
		}
		env.Gas -= gas
		buf := make([]byte, n)
		err := mem.ReadBytes(prog, cpu.GetArg(0), buf, env)
		if err != nil {
			return err
		}
		hs := sha256.Sum256(buf)
		err = mem.WriteBytes(prog, cpu.GetArg(2), hs[:], env)
		if err != nil {
			return err
		}
	case SYSCALL_BALANCE:
		a := AddressType{}
		err := mem.ReadBytes(prog, cpu.GetArg(0), a[:], env)
		if err != nil {
			return err
		}
		ai := GetAccountInfo(ctx.s, a)
		cpu.SetArg(0, ai.Balance)
	case SYSCALL_LOAD_CONTRACT:
		// todo
	case SYSCALL_PROTECTED_CALL:
		// todo
	case SYSCALL_REVERT:
		str, err := mem.ReadString(prog, cpu.GetArg(0), 1024, env)
		if err != nil {
			return err
		}
		gas := uint64(len(str)) * GasSyscallRevertPerByte
		if env.Gas < gas {
			return vm.ErrInsufficientGas
		}
		env.Gas -= gas
		return fmt.Errorf("reverted: %s", str)
	case SYSCALL_TIME:
		cpu.SetArg(0, ctx.ctx.Time)
	case SYSCALL_MINER:
		err := mem.WriteBytes(prog, cpu.GetArg(0), ctx.ctx.Miner[:], env)
		if err != nil {
			return err
		}
	case SYSCALL_BLOCK_NUMBER:
		cpu.SetArg(0, uint64(ctx.ctx.Height))
	case SYSCALL_DIFFICULTY:
		err := mem.WriteBytes(prog, cpu.GetArg(0), ctx.ctx.Difficulty[:], env)
		if err != nil {
			return err
		}
	case SYSCALL_CHAINID:
		cpu.SetArg(0, uint64(ctx.ctx.ChainId))
	case SYSCALL_GAS:
		cpu.SetArg(0, env.Gas)
	case SYSCALL_JUMPDEST:
		target := cpu.GetArg(0)
		if prog != (target >> 32) {
			return ErrIllegalSyscallParameters
		}
		ctx.jumpDest[prog] = true
	}
	return nil
}
