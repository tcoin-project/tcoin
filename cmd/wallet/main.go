package main

import (
	"encoding/hex"
	"fmt"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/utils/address"
)

func main() {
	t, _ := hex.DecodeString("fa91a860d7901c4f1ef23add10e9dee6d468ba267ab271b3b92b8ec365e804db")
	var pubkey block.PubkeyType
	copy(pubkey[:], t)
	addr := block.PubkeyToAddress(pubkey)
	fmt.Println(address.EncodeAddr(addr))
}
