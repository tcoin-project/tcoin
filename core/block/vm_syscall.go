package block

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/vm"
	elfx "github.com/mcfx/tcoin/vm/elf"
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
const SYSCALL_TRANSFER = 19
const SYSCALL_CREATE = 20
const SYSCALL_ED25519_VERIFY = 21
const SYSCALL_LOAD_ELF = 22

const CREATE_TRIMELF = 1
const CREATE_INIT = 2
const CREATE_USENONCE = 4

var ErrInvalidSyscall = errors.New("invalid syscall")
var ErrIllegalSyscallParameters = errors.New("illegal syscall parameters")
var ErrInsufficientBalance = errors.New("insufficient balance")
var ErrContractNotExist = errors.New("contract not exist")
var ErrIllegalEntry = errors.New("illegal entry")
var ErrContractExists = errors.New("contract exists")

const MaxRevertMsgLen = 1024
const MaxByteArrayLen = 1 << 20

var GasSyscallBase = map[int]uint64{
	SYSCALL_SELF:           40,
	SYSCALL_ORIGIN:         40,
	SYSCALL_CALLER:         40,
	SYSCALL_CALLVALUE:      40,
	SYSCALL_STORAGE_STORE:  50000,
	SYSCALL_STORAGE_LOAD:   50000,
	SYSCALL_SHA256:         400,
	SYSCALL_BALANCE:        20000,
	SYSCALL_LOAD_CONTRACT:  500,
	SYSCALL_PROTECTED_CALL: 1000,
	SYSCALL_REVERT:         500,
	SYSCALL_TIME:           40,
	SYSCALL_MINER:          40,
	SYSCALL_BLOCK_NUMBER:   40,
	SYSCALL_DIFFICULTY:     40,
	SYSCALL_CHAINID:        40,
	SYSCALL_GAS:            40,
	SYSCALL_JUMPDEST:       200,
	SYSCALL_TRANSFER:       40000,
	SYSCALL_CREATE:         25000,
	SYSCALL_ED25519_VERIFY: 50000,
	SYSCALL_LOAD_ELF:       500,
}

const GasSyscallSha256PerBlock = 60
const GasSyscallEd25519PerBlock = 100
const GasSyscallRevertPerByte = 1
const GasSyscallTransferMessagePerByte = 1
const GasSyscallCreatePerByte = 1
const GasSyscallCreateStorePerBlock = 10000
const GasLoadContractCodeCached = 400
const GasLoadContractCode = 20000
const GasLoadContractCodePerBlock = 2000

func (ctx *vmCtx) loadContractCode(call *callCtx, addr AddressType) ([]byte, error) {
	env := call.env
	if s, ok := ctx.elfCache[addr]; ok {
		if env.Gas < GasLoadContractCodeCached {
			return nil, vm.ErrInsufficientGas
		}
		env.Gas -= GasLoadContractCodeCached
		return s, nil
	}
	if env.Gas < GasLoadContractCode {
		return nil, vm.ErrInsufficientGas
	}
	env.Gas -= GasLoadContractCode
	key := storage.KeyType{}
	key[0] = 1
	copy(key[1:33], addr[:])
	key[64] = 1
	val := call.s.Read(key)
	if val[0] != 1 {
		return nil, ErrContractNotExist
	}
	n := binary.LittleEndian.Uint64(val[8:16])
	nBlocks := (n + storage.DataLen - 1) / storage.DataLen
	gas := nBlocks * GasLoadContractCodePerBlock
	if env.Gas < gas {
		return nil, vm.ErrInsufficientGas
	}
	env.Gas -= gas
	// todo: optimize consecutive read
	key[0] = 2
	res := make([]byte, nBlocks*storage.DataLen)
	for i := 0; i < int(nBlocks); i++ {
		binary.BigEndian.PutUint64(key[49:65], uint64(i))
		val = call.s.Read(key)
		copy(res[i*storage.DataLen:(i+1)*storage.DataLen], val[:])
	}
	res = res[:n]
	ctx.elfCache[addr] = res
	return res, nil
}

func storeContractCode(s *storage.Slice, addr AddressType, elf []byte) {
	n := len(elf)
	nBlocks := (len(elf) + storage.DataLen - 1) / storage.DataLen
	key := storage.KeyType{}
	key[0] = 1
	copy(key[1:33], addr[:])
	key[64] = 1
	val := storage.DataType{}
	val[0] = 1
	binary.LittleEndian.PutUint64(val[8:16], uint64(n))
	s.Write(key, val)
	key[0] = 2
	for i := 0; i < int(nBlocks); i++ {
		binary.BigEndian.PutUint64(key[49:65], uint64(i))
		if i+1 < int(nBlocks) {
			copy(val[:], elf[i*storage.DataLen:(i+1)*storage.DataLen])
		} else {
			val = storage.DataType{}
			copy(val[:n-i*storage.DataLen], elf[i*storage.DataLen:])
		}
		s.Write(key, val)
	}
}

func (ctx *vmCtx) create(call *callCtx, elf []byte, flags, nonce uint64) (AddressType, error) {
	env := call.env
	mem := ctx.mem
	h := sha256.New()
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, flags)
	h.Write(buf)
	binary.LittleEndian.PutUint64(buf, nonce)
	h.Write(buf)
	h.Write(elf)
	hs := h.Sum(nil)
	addr := AddressType{}
	copy(addr[:], hs)
	id, new, err := ctx.newProgram(addr)
	if err != nil {
		return addr, err
	}
	if !new {
		return addr, ErrContractExists
	}
	key := storage.KeyType{}
	key[0] = 1
	copy(key[1:33], addr[:])
	key[64] = 1
	val := call.s.Read(key)
	if val[0] == 1 {
		return addr, ErrContractExists
	}
	newEntry := ^uint32(0)
	if (flags & CREATE_INIT) != 0 {
		entry, err := mem.Programs[id].LoadELF(elf, 0, env)
		if err != nil {
			return addr, err
		}
		ctx.cpus[id].Reg[2] = DefaultSp
		tEntry, err := ctx.execVM(&callCtx{
			s:         call.s,
			env:       env,
			pc:        uint64(id)<<32 | uint64(entry),
			callValue: 0,
			args:      nil,
			caller:    int(call.prog),
			callType:  CallInit,
		})
		if err != nil {
			return addr, err
		}
		if int(tEntry>>32) != id {
			return addr, ErrIllegalEntry
		}
	}
	if (flags & CREATE_TRIMELF) != 0 {
		e, err := elfx.ParseELF(elf)
		if err != nil {
			return addr, err
		}
		if (^newEntry) == 0 {
			newEntry = e.Entry
		}
		elfNew, err := elfx.TrimELF(elf, e, []uint32{0x100FF000}, uint64(newEntry))
		if err != nil {
			return addr, err
		}
		elf = elfNew
	}
	nBlocks := (len(elf) + storage.DataLen - 1) / storage.DataLen
	gas := uint64(nBlocks * GasSyscallCreateStorePerBlock)
	if env.Gas < gas {
		return addr, vm.ErrInsufficientGas
	}
	env.Gas -= gas
	storeContractCode(call.s, addr, elf)
	return addr, nil
}

func (ctx *vmCtx) execSyscall(call *callCtx, syscallId uint64) error {
	prog := call.prog
	cpu := &ctx.cpus[prog]
	mem := ctx.mem
	env := call.env
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
		err := mem.WriteBytes(prog, cpu.GetArg(0), ctx.addr[call.caller][:], env)
		if err != nil {
			return err
		}
	case SYSCALL_CALLVALUE:
		cpu.SetArg(0, call.callValue)
	case SYSCALL_STORAGE_STORE:
		key := storage.KeyType{}
		val := storage.DataType{}
		key[0] = 2
		copy(key[1:33], ctx.addr[prog][:])
		err := mem.ReadBytes(prog, cpu.GetArg(0), key[33:], env)
		if err != nil {
			return err
		}
		err = mem.ReadBytes(prog, cpu.GetArg(1), val[:], env)
		if err != nil {
			return err
		}
		call.s.Write(key, val)
	case SYSCALL_STORAGE_LOAD:
		key := storage.KeyType{}
		key[0] = 2
		copy(key[1:33], ctx.addr[prog][:])
		err := mem.ReadBytes(prog, cpu.GetArg(0), key[33:], env)
		if err != nil {
			return err
		}
		val := call.s.Read(key)
		err = mem.WriteBytes(prog, cpu.GetArg(1), val[:], env)
		if err != nil {
			return err
		}
	case SYSCALL_SHA256:
		n := cpu.GetArg(1)
		if n > MaxByteArrayLen {
			return ErrIllegalSyscallParameters
		}
		nBlocks := (n + sha256.BlockSize) / sha256.BlockSize
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
		ai := GetAccountInfo(call.s, a)
		cpu.SetArg(0, ai.Balance)
	case SYSCALL_LOAD_CONTRACT:
		a := AddressType{}
		err := mem.ReadBytes(prog, cpu.GetArg(0), a[:], env)
		if err != nil {
			return err
		}
		id, new, err := ctx.newProgram(a)
		if err != nil {
			return err
		}
		if new {
			elf, err := ctx.loadContractCode(call, a)
			if err != nil {
				return err
			}
			entry, err := mem.Programs[id].LoadELF(elf, 0, env)
			if err != nil {
				return err
			}
			ctx.cpus[id].Reg[2] = DefaultSp
			res, err := ctx.execVM(&callCtx{
				s:         call.s,
				env:       env,
				pc:        uint64(id)<<32 | uint64(entry),
				callValue: 0,
				args:      nil,
				caller:    int(prog),
				callType:  CallStart,
			})
			if err != nil {
				return err
			}
			if int(res>>32) != id {
				return ErrIllegalEntry
			}
			ctx.entry[id] = uint32(res)
		}
		cpu.SetArg(0, (uint64(id)<<32)+uint64(ctx.entry[id]))
	case SYSCALL_PROTECTED_CALL:
		callPc := cpu.GetArg(0)
		callValue := cpu.GetArg(3)
		gasLimit := cpu.GetArg(4)
		newS := storage.ForkSlice(call.s)
		callProg := callPc >> 32
		if callProg >= vm.MaxLoadedPrograms {
			return ErrIllegalSyscallParameters
		}
		if callValue != 0 {
			if env.Gas < GasSyscallBase[SYSCALL_TRANSFER] {
				return vm.ErrInsufficientGas
			}
			env.Gas -= GasSyscallBase[SYSCALL_TRANSFER]
			selfInfo := GetAccountInfo(newS, ctx.addr[prog])
			if selfInfo.Balance < callValue {
				return ErrInsufficientBalance
			}
			targetInfo := GetAccountInfo(newS, ctx.addr[callProg])
			selfInfo.Balance -= callValue
			targetInfo.Balance += callValue
			SetAccountInfo(newS, ctx.addr[prog], selfInfo)
			SetAccountInfo(newS, ctx.addr[callProg], targetInfo)
		}
		if ctx.ctx.Callback != nil {
			ctx.ctx.Callback.Transfer(newS, ctx.addr[prog], ctx.addr[callProg], callValue, nil, ctx.tx, ctx.ctx)
		}
		if gasLimit > env.Gas {
			gasLimit = env.Gas
		}
		newEnv := &vm.ExecEnv{
			Gas: gasLimit,
		}
		res, err := ctx.execVM(&callCtx{
			s:         newS,
			env:       newEnv,
			pc:        callPc,
			callValue: callValue,
			args:      []uint64{cpu.GetArg(1), cpu.GetArg(2)},
			caller:    int(prog),
			callType:  CallRegular,
		})
		env.Gas -= gasLimit - newEnv.Gas
		if err != nil {
			err2 := mem.WriteBytes(prog, cpu.GetArg(5), []byte{0}, env)
			if err2 != nil {
				return err2
			}
			err2 = mem.WriteBytes(prog, cpu.GetArg(6), append([]byte(err.Error()), 0), env)
			if err2 != nil {
				return err2
			}
		} else {
			err2 := mem.WriteBytes(prog, cpu.GetArg(5), []byte{1}, env)
			if err2 != nil {
				return err2
			}
			newS.Merge()
			cpu.SetArg(0, res)
		}
	case SYSCALL_REVERT:
		str, err := mem.ReadString(prog, cpu.GetArg(0), MaxRevertMsgLen, env)
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
		ctx.jumpDest[target] = true
	case SYSCALL_TRANSFER:
		addr := AddressType{}
		err := mem.ReadBytes(prog, cpu.GetArg(0), addr[:], env)
		if err != nil {
			return err
		}
		value := cpu.GetArg(1)
		selfInfo := GetAccountInfo(call.s, ctx.addr[prog])
		if selfInfo.Balance < value {
			return ErrInsufficientBalance
		}
		n := cpu.GetArg(3)
		if n > MaxByteArrayLen {
			return ErrIllegalSyscallParameters
		}
		gas := n * GasSyscallTransferMessagePerByte
		if env.Gas < gas {
			return vm.ErrInsufficientGas
		}
		env.Gas -= gas
		buf := make([]byte, n)
		err = mem.ReadBytes(prog, cpu.GetArg(2), buf, env)
		if err != nil {
			return err
		}
		targetInfo := GetAccountInfo(call.s, addr)
		selfInfo.Balance -= value
		targetInfo.Balance += value
		SetAccountInfo(call.s, ctx.addr[prog], selfInfo)
		SetAccountInfo(call.s, addr, targetInfo)
		if ctx.ctx.Callback != nil {
			ctx.ctx.Callback.Transfer(call.s, ctx.addr[prog], addr, value, buf, ctx.tx, ctx.ctx)
		}
	case SYSCALL_CREATE:
		n := cpu.GetArg(2)
		if n > MaxByteArrayLen {
			return ErrIllegalSyscallParameters
		}
		gas := n * GasSyscallCreatePerByte
		if env.Gas < gas {
			return vm.ErrInsufficientGas
		}
		env.Gas -= gas
		buf := make([]byte, n)
		err := mem.ReadBytes(prog, cpu.GetArg(1), buf, env)
		if err != nil {
			return err
		}
		flags := cpu.GetArg(3)
		nonce := cpu.GetArg(4)
		if (flags & CREATE_USENONCE) == 0 {
			key := storage.KeyType{}
			key[0] = 1
			copy(key[1:33], ctx.addr[prog][:])
			key[64] = 2
			val := call.s.Read(key)
			nonce = binary.LittleEndian.Uint64(val[:8])
			binary.LittleEndian.PutUint64(val[:8], nonce+1)
			call.s.Write(key, val)
		}
		addr, err := ctx.create(call, buf, flags, nonce)
		if err != nil {
			return err
		}
		err = mem.WriteBytes(prog, cpu.GetArg(0), addr[:], env)
		if err != nil {
			return err
		}
	case SYSCALL_ED25519_VERIFY:
		n := cpu.GetArg(1)
		if n > MaxByteArrayLen {
			return ErrIllegalSyscallParameters
		}
		nBlocks := (n + sha512.BlockSize - 1) / sha512.BlockSize
		gas := nBlocks * GasSyscallEd25519PerBlock
		if env.Gas < gas {
			return vm.ErrInsufficientGas
		}
		env.Gas -= gas
		buf := make([]byte, n)
		err := mem.ReadBytes(prog, cpu.GetArg(0), buf, env)
		if err != nil {
			return err
		}
		pk := make([]byte, ed25519.PublicKeySize)
		sig := make([]byte, ed25519.SignatureSize)
		err = mem.ReadBytes(prog, cpu.GetArg(2), pk, env)
		if err != nil {
			return err
		}
		err = mem.ReadBytes(prog, cpu.GetArg(3), sig, env)
		if err != nil {
			return err
		}
		var res uint64 = 0
		if ed25519.Verify(pk, buf, sig) {
			res = 1
		}
		cpu.SetArg(0, res)
	case SYSCALL_LOAD_ELF:
		if call.callType != CallInit {
			return ErrIllegalSyscallParameters
		}
		a := AddressType{}
		err := mem.ReadBytes(prog, cpu.GetArg(0), a[:], env)
		if err != nil {
			return err
		}
		elf, err := ctx.loadContractCode(call, a)
		if err != nil {
			return err
		}
		entry, err := mem.Programs[prog].LoadELF(elf, uint32(cpu.GetArg(1)), env)
		if err != nil {
			return err
		}
		cpu.SetArg(0, uint64(entry))
	}
	return nil
}
