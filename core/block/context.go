package block

import "github.com/mcfx/tcoin/storage"

type ExecutionCallback struct {
	Transfer func(s *storage.Slice, from AddressType, to AddressType, value uint64, tx *Transaction, ctx *ExecutionContext)
	Block    func(s *storage.Slice, b *Block, ctx *ExecutionContext)
}

type ExecutionContext struct {
	Height   int
	Time     uint64
	Callback *ExecutionCallback
}
