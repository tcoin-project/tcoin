package block

import "github.com/mcfx/tcoin/storage"

type ExecutionContext struct {
	Transfer func(s *storage.Slice, from AddressType, to AddressType, value uint64)
}
