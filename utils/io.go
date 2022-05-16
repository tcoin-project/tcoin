package utils

type Reader interface {
	Read(p []byte) (n int, err error)
	ReadByte() (c byte, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
	WriteByte(c byte) error
}
