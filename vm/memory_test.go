package vm

import (
	"math/rand"
	"strings"
	"testing"

	elfx "github.com/mcfx/tcoin/vm/elf"
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
			mem := make([]byte, r-l)
			err := m.ReadBytes(0, uint64(base+l), mem, env)
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
	err = m.ReadBytes(0, 0x114514, make([]byte, 100), env)
	assertEq(t, err, ErrSegFault, "expected error")
	env.Gas = 100
	err = m.ReadBytes(0, 0x20000000, make([]byte, 100000), env)
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

func TestLoadTrimedELF(t *testing.T) {
	env := &ExecEnv{
		Gas: 100000000000000000,
	}
	m1 := &Memory{}
	m2 := &Memory{}
	m1.NewProgram()
	m2.NewProgram()
	elf := elfx.DebugBuildELF("__attribute__((section(\".private_data\"))) unsigned long long a[1] = {0xdeadbeef12345678};" +
		"__attribute__((section(\".shared_data\"))) unsigned long long b[512] = {0x0114051419190810};" +
		"__attribute__((section(\".init_code\"))) void *_start() { return start2(); }" +
		"int start2() { return a[0] * b[0]; }")
	e, err := elfx.ParseELF(elf)
	assertEq(t, err, nil, "error happened")
	elf2, err := elfx.TrimELF(elf, e, []uint32{0x100ff000}, 0x10000010)
	assertEq(t, err, nil, "error happened")
	pc1, err := m1.Programs[0].LoadELF(elf, 0, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, pc1, uint32(0x100ff000), "pc mismatch")
	pc2, err := m2.Programs[0].LoadELF(elf2, 0, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, pc2, uint32(0x10000010), "pc mismatch")
	b1 := make([]byte, 4096)
	b2 := make([]byte, 4096)
	err = m1.ReadBytes(0, 0x20000000, b1, env)
	assertEq(t, err, nil, "error happened")
	err = m2.ReadBytes(0, 0x20000000, b2, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, b1, b2, "block 2 not equal")
	err = m1.ReadBytes(0, 0x40000000, b1, env)
	assertEq(t, err, nil, "error happened")
	err = m2.ReadBytes(0, 0x40000000, b2, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, b1, b2, "block 4 not equal")
	err = m1.ReadBytes(0, 0x10000000, b1, env)
	assertEq(t, err, nil, "error happened")
	err = m2.ReadBytes(0, 0x10000000, b2, env)
	assertEq(t, err, nil, "error happened")
	for i := 0; i < 0x190; i++ {
		b1[i] = b2[i]
	}
	assertEq(t, b1, b2, "block 1 not equal")
	m1.Recycle()
	m2.Recycle()
}
