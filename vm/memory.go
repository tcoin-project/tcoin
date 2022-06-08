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
func (m *Memory) ReadBytes(pcProg, ptr, n uint64, env *ExecEnv) []byte {
	chk := func(a *uint64, b bool) bool {
		if b {
			if env.Gas < GasMemoryPage {
				return true
			}
			env.Gas -= GasMemoryPage
		}
		if a == nil {
			return true
		}
		return false
	}
	ptrProg := ptr >> 32
	if pcProg > MaxLoadedPrograms || ptrProg > MaxLoadedPrograms {
		return nil
	}
	isSelf := pcProg == ptrProg
	if n > (1 << 32) {
		return nil
	}
	res := make([]byte, n)
	tbuf := make([]byte, 8)
	cur := 0
	if (ptr & 7) != 0 {
		x, new := m.Programs[ptrProg].Access(uint32(ptr&0xfffffff8), isSelf, OpRead)
		if chk(x, new) {
			return nil
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
		if chk(x, new) {
			return nil
		}
		binary.LittleEndian.PutUint64(res[cur:cur+8], *x)
		cur += 8
	}
	if cur != int(n) {
		x, new := m.Programs[ptrProg].Access(uint32(ptr+uint64(cur)), isSelf, OpRead)
		if chk(x, new) {
			return nil
		}
		c := int(n) - cur
		binary.LittleEndian.PutUint64(tbuf, *x)
		copy(res[cur:cur+c], tbuf[:c])
	}
	return res
}

func (m *Memory) WriteBytes(pcProg, ptr uint64, data []byte, env *ExecEnv) bool {
	chk := func(a *uint64, b bool) bool {
		if b {
			if env.Gas < GasMemoryPage {
				return true
			}
			env.Gas -= GasMemoryPage
		}
		if a == nil {
			return true
		}
		return false
	}
	ptrProg := ptr >> 32
	if pcProg > MaxLoadedPrograms || ptrProg > MaxLoadedPrograms {
		return false
	}
	isSelf := pcProg == ptrProg
	n := len(data)
	if n > (1 << 32) {
		return false
	}
	tbuf := make([]byte, 8)
	cur := 0
	if (ptr & 7) != 0 {
		x, new := m.Programs[ptrProg].Access(uint32(ptr&0xfffffff8), isSelf, OpWrite)
		if chk(x, new) {
			return false
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
		if chk(x, new) {
			return false
		}
		*x = binary.LittleEndian.Uint64(data[cur : cur+8])
		cur += 8
	}
	if cur != int(n) {
		x, new := m.Programs[ptrProg].Access(uint32(ptr+uint64(cur)), isSelf, OpWrite)
		if chk(x, new) {
			return false
		}
		c := int(n) - cur
		binary.LittleEndian.PutUint64(tbuf, *x)
		copy(tbuf[:c], data[cur:cur+c])
		*x = binary.LittleEndian.Uint64(tbuf)
	}
	return true
}
