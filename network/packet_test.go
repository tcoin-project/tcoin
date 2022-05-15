package network

import (
	"bytes"
	"math/rand"
	"reflect"
	"testing"
)

func TestPacketSerialization(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	p := packet{
		tp:   233,
		data: make([]byte, 188889),
	}
	rnd.Read(p.data)
	var b bytes.Buffer
	err := encodePacket(&b, p)
	if err != nil {
		t.Fatal(err)
	}
	p2, err := decodePacket(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p, p2) {
		t.Fatal("not equal")
	}
}
