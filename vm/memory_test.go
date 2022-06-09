package vm

import (
	"math/rand"
	"strings"
	"testing"
)

func TestMemoryAccess(t *testing.T) {
	m := &Memory{}
	id, err := m.NewProgram()
	assertEq(t, err, nil, "error happened")
	assertEq(t, id, 0, "id mismatch")
	x, new := m.Access(0, 0x20000000, OpRead)
	assertUint64Ptr(t, x, false, "should be able to read")
	assertEq(t, new, true, "should allocate")
	x, new = m.Access(1, 0x20000000, OpRead)
	assertUint64Ptr(t, x, true, "shouldn't be able to read")
	assertEq(t, new, false, "shouldn't allocate")
	x, new = m.Access(1, 0x40000000, OpRead)
	assertUint64Ptr(t, x, true, "shouldn't be able to read")
	assertEq(t, new, false, "shouldn't allocate")
	m.Recycle()
}

func TestReadWriteBytes(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	m := &Memory{}
	id, err := m.NewProgram()
	assertEq(t, err, nil, "error happened")
	assertEq(t, id, 0, "id mismatch")
	const n = PageSize * 50
	const base = 0x20000000
	refMem := make([]byte, n)
	env := &ExecEnv{
		Gas: 1000000000000000,
	}
	for i := 0; i < n; i += 8 {
		x, new := m.Access(0, uint64(base+i), OpRead)
		assertEq(t, new, i%PageSize == 0, "allocation mismatch")
		assertEq(t, *x, uint64(0), "not inited")
	}
	for i := 0; i < 1000; i++ {
		l := rand.Intn(n)
		r := rand.Intn(n)
		if l > r {
			l, r = r, l
		}
		r++
		op := rand.Intn(2)
		if op == 0 {
			rnd.Read(refMem[l:r])
			assertEq(t, m.WriteBytes(0, uint64(base+l), refMem[l:r], env), nil, "failed to write")
		} else {
			mem, err := m.ReadBytes(0, uint64(base+l), uint64(r-l), env)
			assertEq(t, err, nil, "read error")
			assertEq(t, mem, refMem[l:r], "read mismatch")
		}
	}
	m.Recycle()
}

func TestReadWriteBytesErrors(t *testing.T) {
	m := &Memory{}
	id, err := m.NewProgram()
	assertEq(t, err, nil, "error happened")
	assertEq(t, id, 0, "id mismatch")
	env := &ExecEnv{
		Gas: 1000000000,
	}
	_, err = m.ReadBytes(0, 0x114514, 100, env)
	assertEq(t, err, ErrSegFault, "expected error")
	env.Gas = 100
	_, err = m.ReadBytes(0, 0x20000000, 100000, env)
	assertEq(t, err, ErrInsufficientGas, "expected error")
	env.Gas = 10000000000
	err = m.WriteBytes(1, 0x40000000, make([]byte, 100), env)
	assertEq(t, err, ErrSegFault, "expected error")
	env.Gas = 100
	err = m.WriteBytes(0, 0x40000000, make([]byte, 100000), env)
	assertEq(t, err, ErrInsufficientGas, "expected error")
	m.Recycle()
}

func TestReadString(t *testing.T) {
	m := &Memory{}
	id, err := m.NewProgram()
	assertEq(t, err, nil, "error happened")
	assertEq(t, id, 0, "id mismatch")
	env := &ExecEnv{
		Gas: 1000000000,
	}
	str := "test_string_123" + strings.Repeat("a", 16000)
	err = m.WriteBytes(0, 0x30000005, []byte(str), env)
	assertEq(t, err, nil, "error happened")
	rs, err := m.ReadString(0, 0x30000005, 100, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, rs, str[:100], "read string mismatch")
	rs, err = m.ReadString(0, 0x30000005, 100000, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, rs, str, "read string mismatch")
	m.Recycle()
}
