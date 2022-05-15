package storage

import (
	"encoding/binary"
	"log"
	"math/rand"
	"os"
	"testing"
)

func testStorage(t *testing.T, storeInMiddle bool) {
	rnd := rand.New(rand.NewSource(114514))
	randK := func() KeyType {
		var k KeyType
		rnd.Read(k[:])
		return k
	}
	randV := func() DataType {
		var v DataType
		rnd.Read(v[:])
		return v
	}
	config := StorageEngineConfig{
		FinalizeDepth: 10,
		DumpDiskRatio: 0.8,
		Path:          "/tmp/tcoin_test/sto_test",
	}
	err := os.RemoveAll("/tmp/tcoin_test/sto_test")
	if err != nil {
		t.Fatal(err)
	}
	k := KeyType{}
	v := DataType{}
	sk := SliceKeyType{}
	sk2 := SliceKeyType{}
	is := EmptySlice()
	is.Write(k, v)
	for i := 0; i < 100; i++ {
		binary.LittleEndian.PutUint64(k[:16], uint64(i))
		binary.LittleEndian.PutUint64(v[:16], uint64(i*i))
		is.Write(k, v)
	}
	binary.LittleEndian.PutUint64(sk[:16], uint64(114514))
	e, err := NewStorageEngine(config, is, sk, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	// build a tree with one long link
	cur := sk
	for i := 1; i <= 100; i++ {
		//log.Printf("build height %d", i)
		a := e.ss[cur]
		b := ForkSlice(a)
		for j := 0; j < rnd.Intn(30); j++ {
			b.Write(randK(), randV())
		}
		b.Freeze()
		binary.LittleEndian.PutUint64(sk[:16], uint64(1000+100*i+1))
		err = e.AddFreezedSlice(b, sk, cur, []byte{1, 2, 3})
		if err != nil {
			t.Fatal(err)
		}
		c := ForkSlice(b)
		for j := 0; j < rnd.Intn(30); j++ {
			c.Write(randK(), randV())
		}
		c.Freeze()
		binary.LittleEndian.PutUint64(sk[:16], uint64(1000+100*i+1))
		binary.LittleEndian.PutUint64(sk2[:16], uint64(1000+100*i+2))
		err = e.AddFreezedSlice(c, sk2, sk, []byte{1, 2, 3})
		if err != nil {
			t.Fatal(err)
		}
		c = ForkSlice(b)
		for j := 0; j < rnd.Intn(30); j++ {
			c.Write(randK(), randV())
		}
		c.Freeze()
		binary.LittleEndian.PutUint64(sk[:16], uint64(1000+100*i+1))
		binary.LittleEndian.PutUint64(sk2[:16], uint64(1000+100*i+3))
		err = e.AddFreezedSlice(c, sk2, sk, []byte{1, 2, 3})
		if err != nil {
			t.Fatal(err)
		}
		b = ForkSlice(a)
		for j := i; j <= 100; j++ {
			binary.LittleEndian.PutUint64(k[:16], uint64(j))
			binary.LittleEndian.PutUint64(v[:16], uint64(i))
			b.Write(k, v)
		}
		b.Freeze()
		binary.LittleEndian.PutUint64(sk[:16], uint64(i))
		err = e.AddFreezedSlice(b, sk, cur, []byte{1, 2, byte(i) + 100})
		if err != nil {
			t.Fatal(err)
		}
		cur = sk
		if storeInMiddle && rnd.Intn(8) == 3 {
			if rnd.Intn(3) == 2 {
				err = e.Flush()
				if err != nil {
					t.Fatal(err)
				}
			}
			e.Stop()
			e, err = NewStorageEngine(config, is, sk, []byte{1, 2, 3})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	err = e.Flush()
	if err != nil {
		t.Fatal(err)
	}
	e.Stop()

	e, err = NewStorageEngine(config, is, sk, []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("ss size: %d", len(e.ss))
	s := e.ss[e.root]
	for i := 1; i <= 80; i++ {
		binary.LittleEndian.PutUint64(k[:16], uint64(i))
		binary.LittleEndian.PutUint64(v[:16], uint64(i))
		if s.Read(k) != v {
			t.Fatalf("wrong: %x %x %x", k, v, s.Read(k))
		}
	}
	for i := 1; i <= 80; i++ {
		k, err := e.ReadKey(i)
		if err != nil {
			t.Fatal(err)
		}
		if binary.LittleEndian.Uint64(k[:16]) != uint64(i) {
			t.Errorf("wrong: %d %x", i, k)
		}
		d, err := e.ReadData(i, SliceKeyType{})
		if err != nil {
			t.Fatal(err)
		}
		if d[2] != byte(i+100) {
			t.Errorf("wrong: %d %x", i, d)
		}
	}
}

func TestStorage1(t *testing.T) {
	testStorage(t, false)
}

func TestStorage2(t *testing.T) {
	testStorage(t, true)
}
