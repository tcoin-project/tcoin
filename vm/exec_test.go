package vm

import (
	"strings"
	"testing"

	elfx "github.com/mcfx/tcoin/vm/elf"
)

func testExecCode(t *testing.T, code []string, targetGas uint64) {
	const InitialGas = 100000000000000
	const RetAddr = 0x0114051419190810
	cpu := &CPU{}
	mem := &Memory{}
	env := &ExecEnv{
		Gas: InitialGas,
	}
	elf := elfx.DebugBuildAsmELF(strings.Join(code, "\n"))
	id, err := mem.NewProgram()
	assertEq(t, id, 0, "program id mismatch")
	assertEq(t, err, nil, "error happened")
	entry, err := mem.Programs[0].LoadELF(elf, 0, env)
	assertEq(t, err, nil, "error happened")
	cpu.SetCall(uint64(entry), RetAddr)
	err = Exec(cpu, mem, env)
	assertEq(t, err, nil, "error happened")
	assertEq(t, InitialGas-env.Gas, targetGas, "consumed gas mismatch")
}

func TestExecBasic(t *testing.T) {
	code := []string{}
	code = append(code, ".section .text")
	code = append(code, ".globl _start")
	code = append(code, "_start:")
	code = append(code, "ret")
	testExecCode(t, code, GasMemoryPage+GasInstructionBase)
}

func TestExecAdvanced(t *testing.T) {
	code := []string{}
	code = append(code, ".section .text")
	code = append(code, ".globl _start")
	code = append(code, "_start:")
	code = append(code, "li a0, 1048576")
	code = append(code, "li a1, 0")
	code = append(code, "li a2, 0x20000000")
	code = append(code, "loop:")
	code = append(code, "add a3, a2, a1")
	code = append(code, "sb a1, 0(a3)")
	code = append(code, "addi a1, a1, 1")
	code = append(code, "bne a1, a0, loop")
	code = append(code, "ret")
	testExecCode(t, code, GasMemoryPage*257+GasInstructionBase*(4+4*1048576)+GasMemoryOp*1048576)
}

func TestExecFail(t *testing.T) {
	code := []string{}
	code = append(code, ".section .text")
	code = append(code, ".globl _start")
	code = append(code, "_start:")
	code = append(code, "li a0, 1048576")
	code = append(code, "li a1, 0")
	code = append(code, "li a2, 0x20000000")
	code = append(code, "loop:")
	code = append(code, "add a3, a2, a1")
	code = append(code, "sb a1, 0(a3)")
	code = append(code, "addi a1, a1, 1")
	code = append(code, "bne a1, a0, loop")
	code = append(code, "ret")
	const InitialGas = 30000000
	const RetAddr = 0x0114051419190810
	cpu := &CPU{}
	mem := &Memory{}
	env := &ExecEnv{
		Gas: InitialGas,
	}
	elf := elfx.DebugBuildAsmELF(strings.Join(code, "\n"))
	id, err := mem.NewProgram()
	assertEq(t, id, 0, "program id mismatch")
	assertEq(t, err, nil, "error happened")
	entry, err := mem.Programs[0].LoadELF(elf, 0, env)
	assertEq(t, err, nil, "error happened")
	cpu.SetCall(uint64(entry), RetAddr)
	err = Exec(cpu, mem, env)
	assertNe(t, err, nil, "expected error")
}
