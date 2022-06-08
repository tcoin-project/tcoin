package vm

import (
	"math/rand"
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
			assertEq(t, m.WriteBytes(0, uint64(base+l), refMem[l:r], env), true, "failed to write")
		} else {
			assertEq(t, m.ReadBytes(0, uint64(base+l), uint64(r-l), env), refMem[l:r], "read mismatch")
		}
	}
}
