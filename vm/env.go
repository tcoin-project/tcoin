package vm

type CPUExecEnv struct {
	Gas       uint64
	MemAccess func(uint64, int) (*uint64, error)
}

type ExecEnv struct {
	Gas uint64
}
