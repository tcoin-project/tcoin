package storage

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestSliceSerilization(t *testing.T) {
	s := EmptySlice()
	k := KeyType{}
	v := DataType{}
	for i := 0; i < 100; i++ {
		binary.LittleEndian.PutUint64(k[:16], uint64(i))
		binary.LittleEndian.PutUint64(v[:16], uint64(i*i))
		s.Write(k, v)
	}
	err := os.MkdirAll("/tmp/tcoin_test", 0o755)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("/tmp/tcoin_test/1.bin")
	if err != nil {
		t.Fatal(err)
	}
	err = s.DumpFile(f)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	f, err = os.Open("/tmp/tcoin_test/1.bin")
	if err != nil {
		t.Fatal(err)
	}
	s2 := EmptySlice()
	err = s2.LoadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	for i := 0; i < 100; i++ {
		binary.LittleEndian.PutUint64(k[:16], uint64(i))
		binary.LittleEndian.PutUint64(v[:16], uint64(i*i))
		if s2.Read(k) != v {
			t.Fatalf("wrong: %x %x %x", k, v, s2.Read(k))
		}
	}
}

func TestSliceBase(t *testing.T) {
	s := EmptySlice()
	s.Freeze()
	k := KeyType{}
	v := DataType{}
	for i := 1; i <= 100; i++ {
		s = ForkSlice(s)
		for j := i; j <= 100; j++ {
			binary.LittleEndian.PutUint64(k[:16], uint64(j))
			binary.LittleEndian.PutUint64(v[:16], uint64(i))
			s.Write(k, v)
		}
	}
	for i := 1; i <= 100; i++ {
		binary.LittleEndian.PutUint64(k[:16], uint64(i))
		binary.LittleEndian.PutUint64(v[:16], uint64(i))
		if s.Read(k) != v {
			t.Fatalf("wrong: %x %x %x", k, v, s.Read(k))
		}
	}
}
