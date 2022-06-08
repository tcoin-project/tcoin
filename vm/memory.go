package vm

type Memory struct {
	programs map[uint32]*ProgramMemory
}
