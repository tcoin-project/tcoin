package vm

import "errors"

var ErrIllegalPc = errors.New("illegal pc")
var ErrSegFault = errors.New("segmentation fault")

func Exec(cpu *CPU, mem *Memory, env *ExecEnv) error {
	pcProg := cpu.Pc >> 32
	cenv := &CPUExecEnv{Gas: env.Gas}
	cenv.MemAccess = func(ptr uint64, op int) (*uint64, error) {
		x, new := mem.Access(pcProg, ptr, op)
		if new {
			if cenv.Gas < GasMemoryPage {
				return nil, ErrInsufficientGas
			}
			cenv.Gas -= GasMemoryPage
		}
		if x == nil {
			return nil, ErrSegFault
		}
		return x, nil
	}
	defer func() {
		env.Gas = cenv.Gas
	}()
	for (cpu.Pc >> 32) == pcProg {
		if (cpu.Pc & 3) != 0 {
			return ErrIllegalPc
		}
		x, new := mem.Access(pcProg, cpu.Pc&^uint64(7), OpExecute)
		if new {
			if cenv.Gas < GasMemoryPage {
				return ErrInsufficientGas
			}
			cenv.Gas -= GasMemoryPage
		}
		if x == nil {
			return ErrSegFault
		}
		insn := uint32(*x >> ((cpu.Pc & 7) * 8))
		nextPc, err := execStep(cpu, cenv, insn)
		if err != nil {
			return err
		}
		cpu.Pc = nextPc
	}
	return nil
}
