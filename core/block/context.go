package block

import "github.com/mcfx/tcoin/storage"

type ExecutionCallback struct {
	// todo: explorer transfer extra data
	Transfer func(s *storage.Slice, from AddressType, to AddressType, value uint64, msg []byte, tx *Transaction, ctx *ExecutionContext)
	Block    func(s *storage.Slice, b *Block, ctx *ExecutionContext)
}

type ExecutionContext struct {
	Height     int
	Time       uint64
	Miner      AddressType
	Difficulty HashType
	ChainId    uint16
	Callback   *ExecutionCallback
}
