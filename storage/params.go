package storage

const KeyLen = 65
const DataLen = 32
const SliceKeyLen = 32
const SliceDataPosLen = 16 + SliceKeyLen

type KeyType [KeyLen]byte
type DataType [DataLen]byte
type SliceKeyType [SliceKeyLen]byte

type StorageEngineConfig struct {
	FinalizeDepth int
	DumpDiskRatio float64
	Path          string
}
