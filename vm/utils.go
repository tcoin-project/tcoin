package vm

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os/exec"
)

func SignExtend32(x uint32) uint64 {
	return uint64(int64(int32(x)))
}

func ImmIType(insn uint32) uint32 {
	return uint32(int32(insn&0xfff00000) >> 20)
}

func ImmSType(insn uint32) uint32 {
	a := insn & 0xfe000000
	b := insn & 0x00000f80
	return uint32(int32(a+(b<<13)) >> 20)
}

func ImmBType(insn uint32) uint32 {
	a := insn & 0x7e000000
	b := insn & 0x00000f00
	c := insn & 0x80000000
	d := insn & 0x00000080
	return uint32(int32(c+(d<<23)+(a>>1)+(b<<12)) >> 19)
}

func ImmUType(insn uint32) uint32 {
	return insn & 0xfffff000
}

func ImmJType(insn uint32) uint32 {
	a := insn & 0x80000000
	b := insn & 0x7fe00000
	c := insn & 0x00100000
	d := insn & 0x000ff000
	return uint32(int32(a+(d<<11)+(c<<2)+(b>>9)) >> 11)
}

func BoolToInt(x bool) uint64 {
	if x {
		return 1
	}
	return 0
}

func checkOpcode(x uint32) {
	if x >= 128 {
		panic(fmt.Sprintf("opcode error: %v", x))
	}
}

func checkReg(x uint32) {
	if x >= 32 {
		panic(fmt.Sprintf("reg error: %v", x))
	}
}

func checkFunct3(x uint32) {
	if x >= 8 {
		panic(fmt.Sprintf("funct3 error: %v", x))
	}
}

func checkFunct7(x uint32) {
	if x >= 128 {
		panic(fmt.Sprintf("funct7 error: %v", x))
	}
}

func genRType(opcode, rd, funct3, rs1, rs2, funct7 uint32) uint32 {
	checkOpcode(opcode)
	checkReg(rd)
	checkReg(rs1)
	checkReg(rs2)
	checkFunct3(funct3)
	checkFunct7(funct7)
	return opcode | rd<<7 | funct3<<12 | rs1<<15 | rs2<<20 | funct7<<25
}

func genIType(opcode, rd, funct3, rs1 uint32, imm int32) uint32 {
	checkOpcode(opcode)
	checkReg(rd)
	checkReg(rs1)
	checkFunct3(funct3)
	if imm < -2048 || imm > 2047 {
		panic(fmt.Sprintf("I-type imm error: %d", imm))
	}
	immt := uint32(imm) & 4095
	return opcode | rd<<7 | funct3<<12 | rs1<<15 | immt<<20
}

func genSType(opcode, funct3, rs1, rs2 uint32, imm int32) uint32 {
	checkOpcode(opcode)
	checkReg(rs1)
	checkReg(rs2)
	checkFunct3(funct3)
	if imm < -2048 || imm > 2047 {
		panic(fmt.Sprintf("S-type imm error: %d", imm))
	}
	immt := uint32(imm) & 4095
	imm_4_0 := immt & 0b11111
	imm_11_5 := immt >> 5
	return opcode | imm_4_0<<7 | funct3<<12 | rs1<<15 | rs2<<20 | imm_11_5<<25
}

func genBType(opcode, funct3, rs1, rs2 uint32, imm int32) uint32 {
	checkOpcode(opcode)
	checkReg(rs1)
	checkReg(rs2)
	checkFunct3(funct3)
	if imm < -4096 || imm > 4095 || imm%2 != 0 {
		panic(fmt.Sprintf("B-type imm error: %d", imm))
	}
	immt := uint32(imm) & 8191
	imm_4_1 := immt >> 1 & 0b1111
	imm_10_5 := immt >> 5 & 0b111111
	imm_11 := immt >> 11 & 1
	imm_12 := immt >> 12 & 1
	imm_4_1_11 := imm_4_1<<1 | imm_11
	imm_12_10_5 := imm_12<<6 | imm_10_5
	return opcode | imm_4_1_11<<7 | funct3<<12 | rs1<<15 | rs2<<20 | imm_12_10_5<<25
}

func genUType(opcode, rd uint32, imm int32) uint32 {
	checkOpcode(opcode)
	checkReg(rd)
	if imm%4096 != 0 {
		panic(fmt.Sprintf("I-type imm error: %d", imm))
	}
	immt := uint32(imm)
	return opcode | rd<<7 | immt
}

func genJType(opcode, rd uint32, imm int32) uint32 {
	checkOpcode(opcode)
	checkReg(rd)
	if imm < -1048576 || imm > 1048575 || imm%2 != 0 {
		panic(fmt.Sprintf("J-type imm error: %d", imm))
	}
	immt := uint32(imm) & 2097151
	imm_10_1 := immt >> 1 & 0b1111111111
	imm_11 := immt >> 11 & 1
	imm_19_12 := immt >> 12 & 0b11111111
	imm_20 := immt >> 20 & 1
	immn := imm_20<<19 | imm_10_1<<9 | imm_11<<8 | imm_19_12
	return opcode | rd<<7 | immn<<12
}

func asmToBytes(asm string) []byte {
	fullAsm := ".section .text\n.globl _start\n_start:\n" + asm + "\n"
	err := ioutil.WriteFile("/tmp/1.S", []byte(fullAsm), 0o755)
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("riscv64-elf-gcc", "/tmp/1.S", "-o", "/tmp/1", "-nostdlib", "-nodefaultlibs", "-march=rv64im", "-mabi=lp64")
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
	cmd = exec.Command("riscv64-elf-objcopy", "-O", "binary", "--only-section=.text", "/tmp/1", "/tmp/2")
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
	res, err := ioutil.ReadFile("/tmp/2")
	if err != nil {
		panic(err)
	}
	return res
}

func asmToInt(asm string) uint32 {
	b := asmToBytes(asm)
	if len(b) != 4 {
		panic("asm length is not 4")
	}
	return binary.LittleEndian.Uint32(b)
}
