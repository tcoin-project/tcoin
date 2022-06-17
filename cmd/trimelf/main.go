package main

import (
	"fmt"
	"io/ioutil"
	"os"

	elfx "github.com/mcfx/tcoin/vm/elf"
)

func main() {
	fn := os.Args[1]
	b, err := ioutil.ReadFile(fn)
	if err != nil {
		panic(err)
	}
	elf, err := elfx.ParseELF(b)
	if err != nil {
		panic(err)
	}
	b2, err := elfx.TrimELF(b, elf, nil, uint64(elf.Entry))
	if err != nil {
		panic(err)
	}
	fmt.Printf("trim: %d -> %d\n", len(b), len(b2))
	ioutil.WriteFile(fn, b2, 0o755)
}
