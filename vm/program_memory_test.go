package vm

import (
	"encoding/binary"
	"io/ioutil"
	"os/exec"
	"testing"
)

func buildELFWithFilename(source, filename, binname string) []byte {
	err := ioutil.WriteFile(filename, []byte(source), 0o755)
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("riscv64-elf-gcc", filename, "-o", binname,
		"-nostdlib", "-nodefaultlibs", "-fno-builtin",
		"-march=rv64im", "-mabi=lp64",
		"-Wl,--gc-sections", "-fPIE", "-s",
		"-Ttext", "0x10000190",
		"-Wl,--section-start,.private_data=0x20000000",
		"-Wl,--section-start,.shared_data=0x40000000",
	)
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
	res, err := ioutil.ReadFile(binname)
	if err != nil {
		panic(err)
	}
	return res
}

func buildELF(source string) []byte {
	return buildELFWithFilename(source, "/tmp/1a.c", "/tmp/1a")
}

func assertUint64Ptr(t *testing.T, a *uint64, isNil bool, msg string, args ...interface{}) {
	if (isNil && a != nil) || (!isNil && a == nil) {
		args = append(args, a)
		args = append(args, isNil)
		t.Fatalf(msg+": got %v, but expected is nil = %v", args...)
	}
}

func TestProgramMemoryPrivileges(t *testing.T) {
	env := &ExecEnv{
		Gas: 100000000000000000,
	}
	pm := ProgramMemory{}
	elf := buildELF("int _start() { return 0; }")
	entry, err := pm.LoadELF(elf, 0, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, entry, uint32(0x10000190), "entry mismatch")
	ptr, new := pm.Access(0x10000000, true, OpRead)
	assertEq(t, new, false, "should be already allocated")
	assertUint64Ptr(t, ptr, false, "privileges error (self read 1)")
	for i := 2; i <= 5; i++ {
		ptr, new = pm.Access(uint32(0x10000000*i), true, OpRead)
		assertEq(t, new, true, "should be unallocated")
		assertUint64Ptr(t, ptr, false, "privileges error (self read %d)", i)
	}
	ptr, new = pm.Access(0x10000000, true, OpExecute)
	assertEq(t, new, false, "should be already allocated")
	assertUint64Ptr(t, ptr, false, "privileges error (self exec 1)")
	for i := 2; i <= 5; i++ {
		ptr, new = pm.Access(uint32(0x10000000*i), true, OpExecute)
		assertEq(t, new, false, "should be already allocated")
		assertUint64Ptr(t, ptr, true, "privileges error (self exec %d)", i)
	}
	ptr, new = pm.Access(0x10000000, true, OpWrite)
	assertEq(t, new, false, "should be already allocated")
	assertUint64Ptr(t, ptr, true, "privileges error (self write 1)")
	for i := 2; i <= 5; i++ {
		ptr, new = pm.Access(uint32(0x10000000*i), true, OpWrite)
		assertEq(t, new, false, "should be already allocated")
		assertUint64Ptr(t, ptr, false, "privileges error (self write %d)", i)
	}
	for i := 1; i <= 5; i++ {
		ptr, new = pm.Access(uint32(0x10000000*i), false, OpRead)
		assertEq(t, new, false, "should be already allocated")
		if i == 2 || i == 3 {
			assertUint64Ptr(t, ptr, true, "privileges error (other read %d)", i)
		} else {
			assertUint64Ptr(t, ptr, false, "privileges error (other read %d)", i)
		}
		ptr, new = pm.Access(uint32(0x10000000*i), false, OpWrite)
		assertEq(t, new, false, "should be already allocated")
		assertUint64Ptr(t, ptr, true, "privileges error (other write %d)", i)
	}
	pm.Recycle()
}

func TestLoadELF(t *testing.T) {
	env := &ExecEnv{
		Gas: 100000000000000000,
	}
	pm := ProgramMemory{}
	elf := buildELF("int _start() { return 0; }")
	entry, err := pm.LoadELF(elf, 0, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, entry, uint32(0x10000190), "entry mismatch")
	elf = buildELF("__attribute__((section(\".private_data\"))) unsigned long long a[1] = {0xdeadbeef12345678};" +
		"__attribute__((section(\".shared_data\"))) unsigned long long b[1] = {0x0114051419190810};" +
		"int _start() { return a[0] ^ b[0]; }")
	_, err = pm.LoadELF(elf, 0, env)
	assertNe(t, err, nil, "error happened")
	entry, err = pm.LoadELF(elf, 0x1000, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, entry, uint32(0x10001190), "entry mismatch")
	ptr, new := pm.Access(0x20001000, true, OpRead)
	assertUint64Ptr(t, ptr, false, "pointer is nil")
	assertEq(t, new, false, "shoule be allocated")
	assertEq(t, *ptr, uint64(0xdeadbeef12345678), "value mismatch")
	ptr, new = pm.Access(0x40001000, true, OpRead)
	assertUint64Ptr(t, ptr, false, "pointer is nil")
	assertEq(t, new, false, "shoule be allocated")
	assertEq(t, *ptr, uint64(0x0114051419190810), "value mismatch")
	for i := 0; i < 10; i++ {
		entry, err = pm.LoadELF(elf, uint32(0x1000*(i+2)), env)
		assertEq(t, err, nil, "error happened")
		assertEq(t, entry, uint32(0x10002190+0x1000*i), "entry mismatch")
	}
	elf = buildELF("__attribute__((section(\".private_data\"))) int _start() {}")
	_, err = pm.LoadELF(elf, 0x30000, env)
	assertNe(t, err, nil, "expected error")
	env.Gas = 0
	elf = buildELF("int _start() { return 0; }")
	_, err = pm.LoadELF(elf, 0, env)
	assertNe(t, err, nil, "expected error")
	pm.Recycle()
}

func TestLoad(t *testing.T) {
	s := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	pm := ProgramMemory{}
	pm.load(0x10000000, 1, append(s, s[:3]...))
	assertEq(t, pm.blocks[0][0][0], binary.LittleEndian.Uint64(s), "value mismatch")
	assertEq(t, pm.blocks[0][0][1], uint64(0x30201), "value mismatch")
}

func TestLoadRawCode(t *testing.T) {
	env := &ExecEnv{
		Gas: 100000000000000000,
	}
	s := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	pm := ProgramMemory{}
	err := pm.LoadRawCode(s, 0x10000000, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, pm.blocks[0][0][0], binary.LittleEndian.Uint64(s), "value mismatch")
}
