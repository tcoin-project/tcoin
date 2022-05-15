package crypto

import (
	"encoding/binary"

	"github.com/mcfx/tcoin/storage"
)

type AccountInfo struct {
	Balance uint64
	Nonce   uint64
}

func GetAccountInfo(s *storage.Slice, address AddressType) AccountInfo {
	key := storage.KeyType{}
	key[0] = 1
	copy(key[1:33], address[:])
	val := s.Read(key)
	return AccountInfo{
		Balance: binary.LittleEndian.Uint64(val[:8]),
		Nonce:   binary.LittleEndian.Uint64(val[8:16]),
	}
}

func SetAccountInfo(s *storage.Slice, address AddressType, info AccountInfo) {
	key := storage.KeyType{}
	key[0] = 1
	copy(key[1:33], address[:])
	val := storage.DataType{}
	binary.LittleEndian.PutUint64(val[:8], info.Balance)
	binary.LittleEndian.PutUint64(val[8:16], info.Nonce)
	s.Write(key, val)
}
