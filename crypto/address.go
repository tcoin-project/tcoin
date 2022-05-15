package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"io"
)

func PubkeyToAddress(pk PubkeyType) AddressType {
	return sha256.Sum256(pk[:])
}

func GenKeyPair(r io.Reader) (a PubkeyType, b PrivkeyType) {
	pubk, prik, err := ed25519.GenerateKey(r)
	if err != nil {
		panic(err)
	}
	copy(a[:], pubk)
	copy(b[:], prik)
	return
}
