package vm

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strconv"
	"strings"
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

type laterInsn struct {
	op   string
	args []string
}

func BuiltinAsmToBytes(asm string) []byte {
	word := func(x string) uint32 {
		r, err := strconv.Atoi(x)
		if err != nil {
			panic(err)
		}
		return uint32(r)
	}
	reg := func(x string) uint32 {
		switch x {
		case "zero":
			return 0
		case "ra":
			return 1
		case "sp":
			return 2
		case "gp":
			return 3
		case "tp":
			return 4
		case "fp":
			return 8
		}
		if x[0] == 'x' {
			return word(x[1:])
		}
		if x[0] == 'a' {
			return 10 + word(x[1:])
		}
		if x[0] == 't' {
			t := word(x[1:])
			if t < 3 {
				return 5 + t
			}
			return 25 + t
		}
		if x[0] == 's' {
			t := word(x[1:])
			if t < 2 {
				return 8 + t
			}
			return 16 + t
		}
		panic(fmt.Sprintf("unknown register: %s", x))
	}
	parseMem := func(x string) (int32, uint32) {
		if x[len(x)-1] != ')' {
			panic(fmt.Sprintf("unknown memory: %s", x))
		}
		t := strings.Split(x[:len(x)-1], "(")
		if len(t) != 2 {
			panic(fmt.Sprintf("unknown memory: %s", x))
		}
		return int32(word(t[0])), reg(t[1])
	}
	rest := []uint32{}
	tbuf := []byte{}
	labels := map[string]int{}
	later := map[int]laterInsn{}
	for _, line := range strings.Split("_start:\n"+asm, "\n") {
		t := strings.Trim(line, " ")
		if len(t) == 0 {
			continue
		}
		if t[len(t)-1] == ':' {
			labels[t[:len(t)-1]] = len(rest)
			continue
		}
		if t[0] == '.' {
			if t[1:6] != "byte " {
				panic("only supports .byte")
			}
			tbuf = append(tbuf, byte(word(t[6:])))
			if len(tbuf) == 4 {
				rest = append(rest, binary.LittleEndian.Uint32(tbuf))
				tbuf = []byte{}
			}
			continue
		}
		if len(tbuf) != 0 {
			panic("insn not aligned to 4")
		}
		var op string
		args := []string{}
		if strings.Contains(line, " ") {
			t2 := strings.SplitN(line, " ", 2)
			op = t2[0]
			for _, k := range strings.Split(t2[1], ",") {
				args = append(args, strings.Trim(k, " "))
			}
		} else {
			op = line
		}
		lin := laterInsn{
			op:   op,
			args: args,
		}
		switch op {
		case "mv":
			rest = append(rest, genIType(0b0010011, reg(args[0]), 0b000, reg(args[1]), 0))
		case "la":
			later[len(rest)] = lin
			rest = append(rest, 0, 0)
		case "li":
			rd := reg(args[0])
			v, err := strconv.Atoi(args[1])
			if err != nil {
				panic(err)
			}
			if v < 2048 && v >= -2048 {
				rest = append(rest, genIType(0b0010011, rd, 0b000, 0, int32(v)))
			} else if v < (1<<31) && v >= -(1<<31) {
				u := v & 0xfff
				if u >= 2048 {
					u -= 4096
				}
				v2 := v - u
				rest = append(rest, genUType(0b0110111, rd, int32(v2)))
				if u != 0 {
					rest = append(rest, genIType(0b0011011, rd, 0b000, rd, int32(u)))
				}
			} else {
				// different from toolchain
				vlo := uint32(uint64(v))
				vhi := uint32(uint64(v) >> 32)
				if len(rest)%2 != 0 {
					rest = append(rest, genIType(0b0010011, 0, 0b000, 0, 0))
				}
				rest = append(rest,
					genUType(0b0010111, rd, 0),
					genJType(0b1101111, 0, 12),
					vlo,
					vhi,
					genIType(0b0000011, rd, 0b011, rd, 8),
				)
			}
		case "srli":
			rest = append(rest, genIType(0b0010011, reg(args[0]), 0b101, reg(args[1]), int32(word(args[2]))))
		case "jalr":
			var rd, rs1 uint32
			if len(args) == 1 {
				rd = 1
				rs1 = reg(args[0])
			} else {
				rd = reg(args[0])
				rs1 = reg(args[1])
			}
			rest = append(rest, genIType(0b1100111, rd, 0b000, rs1, 0))
		case "j":
			later[len(rest)] = lin
			rest = append(rest, 0)
		case "beq":
			later[len(rest)] = lin
			rest = append(rest, 0)
		case "ret":
			rest = append(rest, genIType(0b1100111, 0, 0b000, 1, 0))
		case "addi":
			rest = append(rest, genIType(0b0010011, reg(args[0]), 0b000, reg(args[1]), int32(word(args[2]))))
		case "sub":
			rest = append(rest, genRType(0b0110011, reg(args[0]), 0b000, reg(args[1]), reg(args[2]), 0b0100000))
		case "lb":
			offset, rs1 := parseMem(args[1])
			rest = append(rest, genIType(0b0000011, reg(args[0]), 0b000, rs1, offset))
		case "sb":
			offset, rs1 := parseMem(args[1])
			rest = append(rest, genSType(0b0100011, 0b000, rs1, reg(args[0]), offset))
		case "sd":
			offset, rs1 := parseMem(args[1])
			rest = append(rest, genSType(0b0100011, 0b011, rs1, reg(args[0]), offset))
		default:
			panic(fmt.Sprintf("%s not implemented", op))
		}
	}
	for p, lin := range later {
		_ = p
		_ = lin
		args := lin.args
		switch lin.op {
		case "la":
			diff := (labels[args[1]] - p) * 4
			rd := reg(args[0])
			rest[p] = genUType(0b0010111, rd, 0)
			rest[p+1] = genIType(0b0010011, rd, 0b000, rd, int32(diff))
		case "j":
			diff := (labels[args[0]] - p) * 4
			rest[p] = genJType(0b1101111, 0, int32(diff))
		case "beq":
			diff := (labels[args[2]] - p) * 4
			rest[p] = genBType(0b1100011, 0b000, reg(args[0]), reg(args[1]), int32(diff))
		default:
			panic(fmt.Sprintf("%s not implemented in phase 2", lin.op))
		}
	}
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.LittleEndian, rest)
	if err != nil {
		panic(err)
	}
	return buf.Bytes()
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
		panic(fmt.Sprintf("U-type imm error: %d", imm))
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

func AsmToBytes(asm string) []byte {
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
	b := AsmToBytes(asm)
	if len(b) != 4 {
		panic("asm length is not 4")
	}
	return binary.LittleEndian.Uint32(b)
}
