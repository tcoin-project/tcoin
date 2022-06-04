package vm

type CPUExecEnv struct {
	Gas      uint64
	MemRead  func(uint64) (uint64, error)
	MemWrite func(uint64, uint64) error
	Ecall    func() error
}
