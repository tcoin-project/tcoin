package block

import (
	"crypto/ed25519"
	"crypto/sha256"
)

const AddressLen = sha256.Size
const PubkeyLen = ed25519.PublicKeySize
const PrivkeyLen = ed25519.PrivateKeySize
const SigLen = ed25519.SignatureSize
const HashLen = sha256.Size

type AddressType [AddressLen]byte
type PubkeyType [PubkeyLen]byte
type PrivkeyType [PrivkeyLen]byte
type SigType [SigLen]byte
type HashType [HashLen]byte
