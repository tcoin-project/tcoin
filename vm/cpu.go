package vm

import (
	"errors"
	"math/bits"
)

type CPU struct {
	Reg [32]uint64
	Pc  uint64
}

var ErrIllegalInstruction = errors.New("illegal instruction")
var ErrInsufficientGas = errors.New("insufficient gas")
var ErrDivision = errors.New("division error")
var ErrUnalignedMemoryAccess = errors.New("unaliened memory access")

func execStep(cpu *CPU, env *CPUExecEnv, insn uint32) (uint64, error) {
	cpu.Reg[0] = 0
	nextPc := cpu.Pc + 4
	if env.Gas < GasInstructionBase {
		return nextPc, ErrInsufficientGas
	}
	env.Gas -= GasInstructionBase
	if (insn & 3) != 3 {
		return nextPc, ErrIllegalInstruction
	}

	opcode := insn >> 2 & 0x1f
	rd := insn >> 7 & 0x1f
	funct3 := insn >> 12 & 7
	rs1 := insn >> 15 & 0x1f
	rs2 := insn >> 20 & 0x1f
	rs1v := cpu.Reg[rs1]
	rs2v := cpu.Reg[rs2]
	funct7 := insn >> 25
	switch opcode {
	case 0b01101: // LUI
		cpu.Reg[rd] = SignExtend32(ImmUType(insn))
	case 0b00101: // AUIPC
		cpu.Reg[rd] = SignExtend32(ImmUType(insn)) + cpu.Pc
	case 0b11011: // JAL
		cpu.Reg[rd] = nextPc
		nextPc = SignExtend32(ImmJType(insn)) + cpu.Pc
	case 0b11001: // JALR
		switch funct3 {
		case 0b000:
			tmp := (SignExtend32(ImmIType(insn)) + rs1v) & ^uint64(1)
			cpu.Reg[rd] = nextPc
			nextPc = tmp
		default:
			return nextPc, ErrIllegalInstruction
		}
	case 0b11000: // BRANCH
		var condition bool
		switch funct3 {
		case 0b000: // BEQ
			condition = rs1v == rs2v
		case 0b001: // BNE
			condition = rs1v != rs2v
		case 0b100: // BLT
			condition = int64(rs1v) < int64(rs2v)
		case 0b101: // BGE
			condition = int64(rs1v) >= int64(rs2v)
		case 0b110: // BLTU
			condition = rs1v < rs2v
		case 0b111: // BGEU
			condition = rs1v >= rs2v
		default:
			return nextPc, ErrIllegalInstruction
		}
		if condition {
			nextPc = cpu.Pc + SignExtend32(ImmBType(insn))
		}
	case 0b00000, 0b01000: // LOAD/STORE
		var immTmp uint32
		var op int
		if opcode == 0b00000 {
			immTmp = ImmIType(insn)
			op = OpRead
		} else {
			immTmp = ImmSType(insn)
			op = OpWrite
		}
		addr := rs1v + SignExtend32(immTmp)
		if (addr & ((1 << (funct3 & 3)) - 1)) != 0 {
			return nextPc, ErrUnalignedMemoryAccess
		}
		if env.Gas < GasMemoryOp {
			return nextPc, ErrInsufficientGas
		}
		env.Gas -= GasMemoryOp
		pos, err := env.MemAccess(addr&(^uint64(7)), op)
		if err != nil {
			return nextPc, err
		}
		val := *pos
		offset := (addr & 7) << 3
		if opcode == 0b00000 { // LOAD
			switch funct3 {
			case 0b000: // LB
				cpu.Reg[rd] = uint64(int64(int8(uint8(val >> offset & 0xff))))
			case 0b001: // LH
				cpu.Reg[rd] = uint64(int64(int16(uint16(val >> offset & 0xffff))))
			case 0b010: // LW
				cpu.Reg[rd] = uint64(int64(int32(uint32(val >> offset & 0xffffffff))))
			case 0b011: // LD
				cpu.Reg[rd] = val
			case 0b100: // LBU
				cpu.Reg[rd] = uint64(uint8(val >> offset & 0xff))
			case 0b101: // LHU
				cpu.Reg[rd] = uint64(uint16(val >> offset & 0xffff))
			case 0b110: // LWU
				cpu.Reg[rd] = uint64(uint32(val >> offset & 0xffffffff))
			default:
				return nextPc, ErrIllegalInstruction
			}
		} else { // STORE
			switch funct3 {
			case 0b000: // SB
				val = (val & (^(0xff << offset))) | (uint64(uint8(rs2v)) << offset)
			case 0b001: // SH
				val = (val & (^(0xffff << offset))) | (uint64(uint16(rs2v)) << offset)
			case 0b010: // SW
				val = (val & (^(0xffffffff << offset))) | (uint64(uint32(rs2v)) << offset)
			case 0b011: // SD
				val = rs2v
			default:
				return nextPc, ErrIllegalInstruction
			}
			*pos = val
		}
	case 0b00100: // OP-IMM
		switch funct3 {
		case 0b000: // ADDI
			cpu.Reg[rd] = rs1v + SignExtend32(ImmIType(insn))
		case 0b010: // SLTI
			cpu.Reg[rd] = BoolToInt(int64(rs1v) < int64(SignExtend32(ImmIType(insn))))
		case 0b011: // SLTIU
			cpu.Reg[rd] = BoolToInt(rs1v < SignExtend32(ImmIType(insn)))
		case 0b100: // XORI
			cpu.Reg[rd] = rs1v ^ SignExtend32(ImmIType(insn))
		case 0b110: // ORI
			cpu.Reg[rd] = rs1v | SignExtend32(ImmIType(insn))
		case 0b111: // ANDI
			cpu.Reg[rd] = rs1v & SignExtend32(ImmIType(insn))
		case 0b001, 0b101: // SLLI/SRLI/SRAI
			shamt := insn >> 20 & 0x3f
			tmp := funct7 & 0x7e
			if tmp == 0b0100000 {
				if funct3 != 0b101 {
					return nextPc, ErrIllegalInstruction
				}
				// SRAI
				cpu.Reg[rd] = uint64(int64(rs1v) >> shamt)
			} else if tmp == 0 {
				if funct3 == 0b001 { // SLLI
					cpu.Reg[rd] = rs1v << shamt
				} else { // SRLI
					cpu.Reg[rd] = rs1v >> shamt
				}
			} else {
				return nextPc, ErrIllegalInstruction
			}
		}
	case 0b00110: // OP-IMM-32
		switch funct3 {
		case 0b000: // ADDIW
			cpu.Reg[rd] = SignExtend32(uint32(rs1v) + ImmIType(insn))
		case 0b001: // SLLIW
			if funct7 != 0 {
				return nextPc, ErrIllegalInstruction
			}
			cpu.Reg[rd] = SignExtend32(uint32(rs1v) << rs2)
		case 0b101: // SRLIW/SRAIW
			if funct7 == 0b0100000 { // SRAIW
				cpu.Reg[rd] = SignExtend32(uint32(int32(uint32(rs1v)) >> rs2))
			} else if funct7 == 0 { // SRLIW
				cpu.Reg[rd] = SignExtend32(uint32(rs1v) >> rs2)
			} else {
				return nextPc, ErrIllegalInstruction
			}
		default:
			return nextPc, ErrIllegalInstruction
		}
	case 0b01100: // OP
		if funct7 == 0b0000001 { // MULDIV
			switch funct3 {
			case 0b000: // MUL
				cpu.Reg[rd] = rs1v * rs2v
			case 0b001: // MULH
				hi, _ := bits.Mul64(rs1v, rs2v)
				if (rs1v >> 63) != 0 {
					hi -= rs2v
				}
				if (rs2v >> 63) != 0 {
					hi -= rs1v
				}
				cpu.Reg[rd] = hi
			case 0b010: // MULHSU
				hi, _ := bits.Mul64(rs1v, rs2v)
				if (rs1v >> 63) != 0 {
					hi -= rs2v
				}
				cpu.Reg[rd] = hi
			case 0b011: // MULHU
				hi, _ := bits.Mul64(rs1v, rs2v)
				cpu.Reg[rd] = hi
			case 0b100, 0b110: // DIV/REM
				if rs2v == 0 || (rs1v == (1<<63) && rs2v+1 == 0) {
					return nextPc, ErrDivision
				}
				if funct3 == 0b100 { // DIV
					cpu.Reg[rd] = uint64(int64(rs1v) / int64(rs2v))
				} else { // REM
					cpu.Reg[rd] = uint64(int64(rs1v) % int64(rs2v))
				}
			case 0b101, 0b111: // DIVU/REMU
				if rs2v == 0 {
					return nextPc, ErrDivision
				}
				if funct3 == 0b101 { // DIVU
					cpu.Reg[rd] = rs1v / rs2v
				} else { // REMU
					cpu.Reg[rd] = rs1v % rs2v
				}
			}
		} else {
			if funct7 == 0b0100000 {
				if funct3 != 0b000 && funct3 != 0b101 {
					return nextPc, ErrIllegalInstruction
				}
			} else if funct7 != 0 {
				return nextPc, ErrIllegalInstruction
			}
			switch funct3 {
			case 0b000: // ADD/SUB
				if funct7 == 0b0100000 { // SUB
					cpu.Reg[rd] = rs1v - rs2v
				} else { // ADD
					cpu.Reg[rd] = rs1v + rs2v
				}
			case 0b001: // SLL
				cpu.Reg[rd] = rs1v << (rs2v & 0x3f)
			case 0b010: // SLT
				cpu.Reg[rd] = BoolToInt(int64(rs1v) < int64(rs2v))
			case 0b011: // SLTU
				cpu.Reg[rd] = BoolToInt(rs1v < rs2v)
			case 0b100: // XOR
				cpu.Reg[rd] = rs1v ^ rs2v
			case 0b101: // SRL/SRA
				if funct7 == 0b0100000 { // SRA
					cpu.Reg[rd] = uint64(int64(rs1v) >> (rs2v & 0x3f))
				} else { // SRL
					cpu.Reg[rd] = rs1v >> (rs2v & 0x3f)
				}
			case 0b110: // OR
				cpu.Reg[rd] = rs1v | rs2v
			case 0b111: // AND
				cpu.Reg[rd] = rs1v & rs2v
			}
		}
	case 0b01110: // OP-32
		if funct7 == 0b0000001 { // MULDIV
			switch funct3 {
			case 0b000: // MULW
				cpu.Reg[rd] = SignExtend32(uint32(rs1v * rs2v))
			case 0b100, 0b101, 0b110, 0b111: // DIVW/DIVUW/REMW/REMUW
				rs1u := uint32(rs1v)
				rs2u := uint32(rs2v)
				if rs2u == 0 {
					return nextPc, ErrDivision
				}
				switch funct3 {
				case 0b100, 0b110: // DIVW/REMW
					if rs1u == (1<<31) && rs2u+1 == 0 {
						return nextPc, ErrDivision
					}
					if funct3 == 0b100 { // DIVW
						cpu.Reg[rd] = SignExtend32(uint32(int32(rs1u) / int32(rs2u)))
					} else { // REM
						cpu.Reg[rd] = SignExtend32(uint32(int32(rs1u) % int32(rs2u)))
					}
				case 0b101, 0b111: // DIVUW/REMUW
					if funct3 == 0b101 { // DIVU
						cpu.Reg[rd] = SignExtend32(rs1u / rs2u)
					} else { // REMU
						cpu.Reg[rd] = SignExtend32(rs1u % rs2u)
					}
				}
			default:
				return nextPc, ErrIllegalInstruction
			}
		} else {
			if funct7 == 0b0100000 {
				if funct3 != 0b000 && funct3 != 0b101 {
					return nextPc, ErrIllegalInstruction
				}
			} else if funct7 != 0 {
				return nextPc, ErrIllegalInstruction
			}
			switch funct3 {
			case 0b000: // ADDW/SUBW
				if funct7 == 0b0100000 { // SUBW
					cpu.Reg[rd] = SignExtend32(uint32(rs1v - rs2v))
				} else { // ADDW
					cpu.Reg[rd] = SignExtend32(uint32(rs1v + rs2v))
				}
			case 0b001: // SLLW
				cpu.Reg[rd] = SignExtend32(uint32(rs1v) << (rs2v & 0x1f))
			case 0b101: // SRLW/SRAW
				if funct7 == 0b0100000 { // SRAW
					cpu.Reg[rd] = SignExtend32(uint32(int32(uint32(rs1v)) >> (rs2v & 0x1f)))
				} else { // SRLW
					cpu.Reg[rd] = SignExtend32(uint32(rs1v) >> (rs2v & 0x1f))
				}
			default:
				return nextPc, ErrIllegalInstruction
			}
		}
	default:
		return nextPc, ErrIllegalInstruction
	}
	cpu.Pc = nextPc
	return nextPc, nil
}

func (c *CPU) SetCall(target, ret uint64) {
	c.Pc = target
	c.Reg[1] = ret
}

func (c *CPU) GetArg(i int) uint64 {
	return c.Reg[10+i]
}

func (c *CPU) SetArg(i int, val uint64) {
	c.Reg[10+i] = val
}

func (c *CPU) Ret() {
	c.Pc = c.Reg[1]
}
