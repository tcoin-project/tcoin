package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Slice struct {
	base    *Slice
	height  int
	st      map[KeyType]DataType
	freezed bool
}

func EmptySlice() *Slice {
	return &Slice{
		base:    nil,
		height:  0,
		st:      make(map[KeyType]DataType),
		freezed: false,
	}
}

func ForkSlice(base *Slice) *Slice {
	return &Slice{
		base:    base,
		height:  base.height + 1,
		st:      make(map[KeyType]DataType),
		freezed: false,
	}
}

func (s *Slice) Height() int {
	return s.height
}

func (s *Slice) Merge() {
	for k, v := range s.st {
		s.base.st[k] = v
	}
}

func (s *Slice) Read(k KeyType) DataType {
	u := s
	for {
		if v, ok := u.st[k]; ok {
			return v
		}
		if u.base == nil {
			return DataType{}
		}
		u = u.base
	}
}

func (s *Slice) Write(k KeyType, v DataType) {
	if s.freezed {
		panic(errors.New("write to freezed slice"))
	}
	s.st[k] = v
}

func (s *Slice) Freeze() {
	s.freezed = true
}

func (s *Slice) LoadFile(f io.Reader) error {
	if s.freezed {
		return errors.New("load to a freezed slice")
	}
	s.freezed = true
	if len(s.st) > 0 {
		return errors.New("load to a nonempty slice")
	}
	lbuf := make([]byte, 8)
	_, err := io.ReadFull(f, lbuf)
	if err != nil {
		return fmt.Errorf("error when loading slice: %v", err)
	}
	s.height = int(binary.LittleEndian.Uint64(lbuf))
	_, err = io.ReadFull(f, lbuf)
	if err != nil {
		return fmt.Errorf("error when loading slice: %v", err)
	}
	cnt := int(binary.LittleEndian.Uint64(lbuf))
	var k KeyType
	var v DataType
	for i := 0; i < cnt; i++ {
		_, err = io.ReadFull(f, k[:])
		if err != nil {
			return fmt.Errorf("error when loading slice: %v", err)
		}
		_, err = io.ReadFull(f, v[:])
		if err != nil {
			return fmt.Errorf("error when loading slice: %v", err)
		}
		s.st[k] = v
	}
	return nil
}

func (s *Slice) DumpFile(f io.Writer) error {
	lbuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(lbuf, uint64(s.height))
	_, err := f.Write(lbuf)
	if err != nil {
		return fmt.Errorf("error when dumping slice: %v", err)
	}
	binary.LittleEndian.PutUint64(lbuf, uint64(len(s.st)))
	_, err = f.Write(lbuf)
	if err != nil {
		return fmt.Errorf("error when dumping slice: %v", err)
	}
	for k, v := range s.st {
		_, err = f.Write(k[:])
		if err != nil {
			return fmt.Errorf("error when dumping slice: %v", err)
		}
		_, err = f.Write(v[:])
		if err != nil {
			return fmt.Errorf("error when dumping slice: %v", err)
		}
	}
	return nil
}
