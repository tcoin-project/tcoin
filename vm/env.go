package vm

type CPUExecEnv struct {
	Gas     uint64
	MemRead func(uint64) (*uint64, error)
}
