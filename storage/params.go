package storage

import "crypto/sha256"

const KeyLen = 65
const DataLen = 32
const SliceKeyLen = sha256.Size
const SliceDataPosLen = 16 + SliceKeyLen

type KeyType [KeyLen]byte
type DataType [DataLen]byte
type SliceKeyType [SliceKeyLen]byte

type StorageEngineConfig struct {
	FinalizeDepth int
	DumpDiskRatio float64
	Path          string
}
