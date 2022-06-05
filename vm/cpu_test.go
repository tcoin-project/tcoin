package vm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func genPrepareCode() ([]string, int) {
	res := []string{}
	offset := 0
	res = append(res, ".section .text")
	res = append(res, ".globl _start")
	res = append(res, "setup_regs:")
	for i := 4; i < 32; i++ {
		res = append(res, fmt.Sprintf("ld x%d, %d(x3)", i, (i-3)*8))
		offset += 4
	}
	res = append(res, "ret")
	offset += 4
	res = append(res, "print_regs:")
	for i := 4; i < 32; i++ {
		res = append(res, fmt.Sprintf("sd x%d, %d(sp)", i, (i-32)*8))
		offset += 4
	}
	res = append(res, "li a0, 1")
	res = append(res, fmt.Sprintf("addi a1, sp, %d", -28*8))
	res = append(res, fmt.Sprintf("li a2, %d", 28*8))
	res = append(res, "li a7, 64")
	res = append(res, "ecall")
	res = append(res, "ret")
	res = append(res, "_start:")
	offset += 4 * 6
	return res, offset
}

func genFinishCode(res []string) []string {
	res = append(res, "li a0, 0")
	res = append(res, "li a7, 93")
	res = append(res, "ecall")
	return res
}

func addTestCase(code []string, regs []uint64, s, label string) ([]string, int, int) {
	offset := 0
	code = append(code, "auipc x3, 0")
	code = append(code, "j next_"+label)
	offset += 4 * 2
	for _, x := range regs {
		for i := 0; i < 8; i++ {
			code = append(code, fmt.Sprintf(".byte %d", x>>(i*8)&255))
		}
		offset += 8
	}
	code = append(code, "next_"+label+":")
	code = append(code, "jal setup_regs")
	offset += 4
	code = append(code, s)
	code = append(code, "jal print_regs")
	return code, offset, offset + 4*2
}

func testBatchInsns(t *testing.T, regs [][]uint64, insns []string) {
	var codeOffset, codeSize int
	code, initOffset := genPrepareCode()
	for i := 0; i < len(regs); i++ {
		code, codeOffset, codeSize = addTestCase(code, regs[i], insns[i], strconv.Itoa(i))
	}
	code = genFinishCode(code)
	err := ioutil.WriteFile("/tmp/3.S", []byte(strings.Join(code, "\n")), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("riscv64-elf-gcc", "/tmp/3.S", "-o", "/tmp/3", "-nostdlib", "-nodefaultlibs", "-march=rv64im", "-mabi=lp64", "-Ttext", "0x100000")
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("qemu-riscv64", "/tmp/3")
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	bs := buf.Bytes()
	asm := asmToBytes(strings.Join(insns, "\n"))
	if len(asm) != len(regs)*4 {
		t.Fatal("asm length invalid")
	}
	for T := 0; T < len(regs); T++ {
		insn := binary.LittleEndian.Uint32(asm[T*4 : T*4+4])
		cpu := &CPU{}
		for i := 4; i < 32; i++ {
			cpu.Reg[i] = regs[T][i-4]
		}
		cpu.Pc = uint64(0x100000 + initOffset + codeSize*T + codeOffset)
		env := &CPUExecEnv{
			Gas:     100,
			MemRead: nil,
		}
		_, err := execStep(cpu, env, insn)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 28; i++ {
			actual := binary.LittleEndian.Uint64(bs[T*28*8+i*8 : T*28*8+i*8+8])
			expected := cpu.Reg[i+4]
			if actual != expected {
				t.Fatalf("test %d (%s) x%d mismatch: got %v, expected %v", T, insns[T], i+4, actual, expected)
			}
		}
	}
}

func TestOps(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	randRegs := func() []uint64 {
		res := []uint64{}
		for i := 0; i < 28; i++ {
			res = append(res, rnd.Uint64())
		}
		return res
	}
	getReg := func() int {
		return rnd.Intn(28) + 4
	}
	genUType := func(x string) func() string {
		return func() string {
			return fmt.Sprintf("%s x%d, %d", x, getReg(), rnd.Intn(1048576))
		}
	}
	genIType := func(x string) func() string {
		return func() string {
			return fmt.Sprintf("%s x%d, x%d, %d", x, getReg(), getReg(), rnd.Intn(4096)-2048)
		}
	}
	genRType := func(x string) func() string {
		return func() string {
			return fmt.Sprintf("%s x%d, x%d, x%d", x, getReg(), getReg(), getReg())
		}
	}
	genShift := func(x string) func() string {
		return func() string {
			return fmt.Sprintf("%s x%d, x%d, %d", x, getReg(), getReg(), rnd.Intn(64))
		}
	}
	genShift32 := func(x string) func() string {
		return func() string {
			return fmt.Sprintf("%s x%d, x%d, %d", x, getReg(), getReg(), rnd.Intn(32))
		}
	}
	genList := []func() string{
		genUType("lui"),
		genUType("auipc"),
		genIType("addi"),
		genIType("slti"),
		genIType("sltiu"),
		genIType("xori"),
		genIType("ori"),
		genIType("andi"),
		genShift("slli"),
		genShift("srli"),
		genShift("srai"),
		genRType("add"),
		genRType("sub"),
		genRType("sll"),
		genRType("slt"),
		genRType("sltu"),
		genRType("xor"),
		genRType("srl"),
		genRType("sra"),
		genRType("or"),
		genRType("and"),
		genIType("addiw"),
		genShift32("slliw"),
		genShift32("srliw"),
		genShift32("sraiw"),
		genRType("addw"),
		genRType("subw"),
		genRType("sllw"),
		genRType("srlw"),
		genRType("sraw"),
		genRType("mul"),
		genRType("mulh"),
		genRType("mulhsu"),
		genRType("mulhu"),
		genRType("div"),
		genRType("divu"),
		genRType("rem"),
		genRType("remu"),
		genRType("mulw"),
		genRType("divw"),
		genRType("divuw"),
		genRType("remw"),
		genRType("remuw"),
	}
	const batchSize = 10
	for l := 0; l < len(genList); l += batchSize {
		regs := [][]uint64{}
		insns := []string{}
		for i := l; i < len(genList) && i < l+batchSize; i++ {
			for j := 0; j < 200; j++ {
				regs = append(regs, randRegs())
				insns = append(insns, genList[i]())
			}
		}
		testBatchInsns(t, regs, insns)
	}
}

func TestLoadStore(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	randInts := func(n int) []uint64 {
		res := []uint64{}
		for i := 0; i < n; i++ {
			res = append(res, rnd.Uint64())
		}
		return res
	}
	getReg := func() int {
		return rnd.Intn(28) + 4
	}
	code, offset := genPrepareCode()
	initialRegs := randInts(28)
	initialMem := randInts(512)
	code = append(code, "auipc x3, 0")
	code = append(code, "j next")
	offset += 8
	for _, x := range initialRegs {
		for i := 0; i < 8; i++ {
			code = append(code, fmt.Sprintf(".byte %d", x>>(i*8)&255))
		}
		offset += 8
	}
	code = append(code, "next:")
	code = append(code, "jal setup_regs")
	code = append(code, "li x3, 33556480")
	offset += 12

	const TestCount = 10000
	ops := []string{"lb", "lh", "lw", "ld", "lbu", "lhu", "lwu", "sb", "sh", "sw", "sd"}
	alignment := []int{1, 2, 4, 8, 1, 2, 4, 1, 2, 4, 8}
	codeOffset := len(code)
	for i := 0; i < TestCount; i++ {
		op := rnd.Intn(len(ops))
		var addr int
		for {
			addr = rnd.Intn(4096) - 2048
			if addr%alignment[op] == 0 {
				break
			}
		}
		code = append(code, fmt.Sprintf("%s x%d, %d(x3)", ops[op], getReg(), addr))
	}
	code = append(code, "jal print_regs")
	code = append(code, "li a0, 1")
	code = append(code, "la a1, array")
	code = append(code, "li a2, 4096")
	code = append(code, "li a7, 64")
	code = append(code, "ecall")
	code = genFinishCode(code)
	code = append(code, ".section .data")
	code = append(code, "array:")
	for pos, x := range initialMem {
		if pos == 256 {
			code = append(code, "array_2048:")
		}
		for i := 0; i < 8; i++ {
			code = append(code, fmt.Sprintf(".byte %d", x>>(i*8)&255))
		}
	}

	err := ioutil.WriteFile("/tmp/5.S", []byte(strings.Join(code, "\n")), 0o755)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("riscv64-elf-gcc", "/tmp/5.S", "-o", "/tmp/5", "-nostdlib", "-nodefaultlibs", "-march=rv64im", "-mabi=lp64", "-Ttext", "0x1000000", "-Tdata", "0x2000000")
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("riscv64-elf-objcopy", "-O", "binary", "--only-section=.text", "/tmp/5", "/tmp/6")
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	asm, err := ioutil.ReadFile("/tmp/6")
	if err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command("qemu-riscv64", "/tmp/5")
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	err = cmd.Run()
	if err != nil {
		t.Fatal(err)
	}
	bs := buf.Bytes()
	_ = bs

	const memBase = 0x100000000000
	mem := make([]uint64, 512)
	copy(mem, initialMem)
	cpu := &CPU{}
	for i := 4; i < 32; i++ {
		cpu.Reg[i] = initialRegs[i-4]
	}
	cpu.Reg[3] = memBase + 2048
	env := &CPUExecEnv{
		Gas: 1000000000000,
		MemRead: func(u uint64) (*uint64, error) {
			if u < memBase || u >= memBase+4096 {
				t.Fatalf("memory access out of range: %d", u)
			}
			if (u & 7) != 0 {
				t.Fatalf("memory access not aligned: %d", u)
			}
			return &mem[(u-memBase)>>3], nil
		},
	}
	for i := 0; i < TestCount; i++ {
		insn := binary.LittleEndian.Uint32(asm[i*4+offset : i*4+4+offset])
		_, err := execStep(cpu, env, insn)
		if err != nil {
			t.Fatalf("%d %s error: %v", i, code[codeOffset+i], err)
		}
	}
	for i := 0; i < 28+512; i++ {
		actual := binary.LittleEndian.Uint64(bs[i*8 : i*8+8])
		var expected uint64
		if i < 28 {
			expected = cpu.Reg[i+4]
		} else {
			expected = mem[i-28]
		}
		if actual != expected {
			t.Fatalf("%d mismatch: got %v, expected %v", i, actual, expected)
		}
	}
}

// todo: jal, jalr
// todo: beq, bne, blt, bge, bltu, bgeu
