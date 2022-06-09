package vm

import (
	"encoding/binary"
	"errors"
)

const MaxLoadedPrograms = 256

var ErrTooManyPrograms = errors.New("too many Programs")

type Memory struct {
	Programs [MaxLoadedPrograms]*ProgramMemory
}

func (m *Memory) NewProgram() (int, error) {
	for i := 0; i < MaxLoadedPrograms; i++ {
		if m.Programs[i] == nil {
			m.Programs[i] = pmPool.Get().(*ProgramMemory)
			return i, nil
		}
	}
	return 0, ErrTooManyPrograms
}

func (m *Memory) Recycle() {
	for i := 0; i < MaxLoadedPrograms; i++ {
		if m.Programs[i] != nil {
			m.Programs[i].Recycle()
			pmPool.Put(m.Programs[i])
			m.Programs[i] = nil
		}
	}
}

func (m *Memory) Access(pcProg, ptr uint64, op int) (*uint64, bool) {
	ptrProg := ptr >> 32
	if pcProg > MaxLoadedPrograms || ptrProg > MaxLoadedPrograms {
		return nil, false
	}
	return m.Programs[ptrProg].Access(uint32(ptr), pcProg == ptrProg, op)
}

// todo: optimize consecutive memory access
func (m *Memory) ReadBytes(pcProg, ptr, n uint64, env *ExecEnv) ([]byte, error) {
	chk := func(a *uint64, b bool) error {
		if b {
			if env.Gas < GasMemoryPage {
				return ErrInsufficientGas
			}
			env.Gas -= GasMemoryPage
		}
		if a == nil {
			return ErrSegFault
		}
		return nil
	}
	ptrProg := ptr >> 32
	if pcProg > MaxLoadedPrograms || ptrProg > MaxLoadedPrograms {
		return nil, ErrSegFault
	}
	isSelf := pcProg == ptrProg
	if n > (1 << 32) {
		return nil, ErrSegFault
	}
	res := make([]byte, n)
	tbuf := make([]byte, 8)
	cur := 0
	if (ptr & 7) != 0 {
		x, new := m.Programs[ptrProg].Access(uint32(ptr&0xfffffff8), isSelf, OpRead)
		if err := chk(x, new); err != nil {
			return nil, err
		}
		binary.LittleEndian.PutUint64(tbuf, *x)
		cur = 8 - int(ptr&7)
		if cur > int(n) {
			cur = int(n)
		}
		copy(res[:cur], tbuf[ptr&7:int(ptr&7)+cur])
	}
	for cur+8 <= int(n) {
		x, new := m.Programs[ptrProg].Access(uint32(ptr+uint64(cur)), isSelf, OpRead)
		if err := chk(x, new); err != nil {
			return nil, err
		}
		binary.LittleEndian.PutUint64(res[cur:cur+8], *x)
		cur += 8
	}
	if cur != int(n) {
		x, new := m.Programs[ptrProg].Access(uint32(ptr+uint64(cur)), isSelf, OpRead)
		if err := chk(x, new); err != nil {
			return nil, err
		}
		c := int(n) - cur
		binary.LittleEndian.PutUint64(tbuf, *x)
		copy(res[cur:cur+c], tbuf[:c])
	}
	return res, nil
}

func (m *Memory) WriteBytes(pcProg, ptr uint64, data []byte, env *ExecEnv) error {
	chk := func(a *uint64, b bool) error {
		if b {
			if env.Gas < GasMemoryPage {
				return ErrInsufficientGas
			}
			env.Gas -= GasMemoryPage
		}
		if a == nil {
			return ErrSegFault
		}
		return nil
	}
	ptrProg := ptr >> 32
	if pcProg > MaxLoadedPrograms || ptrProg > MaxLoadedPrograms {
		return ErrSegFault
	}
	isSelf := pcProg == ptrProg
	n := len(data)
	if n > (1 << 32) {
		return ErrSegFault
	}
	tbuf := make([]byte, 8)
	cur := 0
	if (ptr & 7) != 0 {
		x, new := m.Programs[ptrProg].Access(uint32(ptr&0xfffffff8), isSelf, OpWrite)
		if err := chk(x, new); err != nil {
			return err
		}
		binary.LittleEndian.PutUint64(tbuf, *x)
		cur = 8 - int(ptr&7)
		if cur > int(n) {
			cur = int(n)
		}
		copy(tbuf[ptr&7:int(ptr&7)+cur], data[:cur])
		*x = binary.LittleEndian.Uint64(tbuf)
	}
	for cur+8 <= int(n) {
		x, new := m.Programs[ptrProg].Access(uint32(ptr+uint64(cur)), isSelf, OpWrite)
		if err := chk(x, new); err != nil {
			return err
		}
		*x = binary.LittleEndian.Uint64(data[cur : cur+8])
		cur += 8
	}
	if cur != int(n) {
		x, new := m.Programs[ptrProg].Access(uint32(ptr+uint64(cur)), isSelf, OpWrite)
		if err := chk(x, new); err != nil {
			return err
		}
		c := int(n) - cur
		binary.LittleEndian.PutUint64(tbuf, *x)
		copy(tbuf[:c], data[cur:cur+c])
		*x = binary.LittleEndian.Uint64(tbuf)
	}
	return nil
}

func (m *Memory) ReadString(pcProg, ptr, maxLen uint64, env *ExecEnv) (string, error) {
	chk := func(a *uint64, b bool) error {
		if b {
			if env.Gas < GasMemoryPage {
				return ErrInsufficientGas
			}
			env.Gas -= GasMemoryPage
		}
		if a == nil {
			return ErrSegFault
		}
		return nil
	}
	ptrProg := ptr >> 32
	if pcProg > MaxLoadedPrograms || ptrProg > MaxLoadedPrograms {
		return "", ErrSegFault
	}
	isSelf := pcProg == ptrProg
	res := []byte{}
	tbuf := make([]byte, 8)
	lst := 0
	chkRes := func() bool {
		for ; lst < len(res); lst++ {
			if res[lst] == 0 || lst == int(maxLen) {
				res = res[:lst]
				return true
			}
		}
		return false
	}
	if (ptr & 7) != 0 {
		x, new := m.Programs[ptrProg].Access(uint32(ptr&0xfffffff8), isSelf, OpRead)
		if err := chk(x, new); err != nil {
			return "", err
		}
		binary.LittleEndian.PutUint64(tbuf, *x)
		c := 8 - int(ptr&7)
		res = make([]byte, c)
		copy(res, tbuf[ptr&7:int(ptr&7)+c])
	}
	for {
		if chkRes() {
			return string(res), nil
		}
		x, new := m.Programs[ptrProg].Access(uint32(ptr+uint64(len(res))), isSelf, OpRead)
		if err := chk(x, new); err != nil {
			return "", err
		}
		binary.LittleEndian.PutUint64(tbuf, *x)
		res = append(res, tbuf...)
	}
}
