package address

import (
	"math/rand"
	"testing"

	"github.com/mcfx/tcoin/core/block"
)

func TestAddress(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	var addr block.AddressType
	for i := 0; i < 114514; i++ {
		rnd.Read(addr[:])
		x := EncodeAddr(addr)
		u, err := ParseAddr(x)
		if err != nil {
			t.Fatal(err)
		}
		if u != addr {
			t.Fatal("addr mismatch")
		}
	}
}
