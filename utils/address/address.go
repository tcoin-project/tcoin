package address

import (
	"errors"

	"github.com/mcfx/tcoin/core/block"

	"github.com/mr-tron/base58"
)

func checkSum(addr block.AddressType) byte {
	var res byte = 0
	for _, x := range addr {
		res += x
	}
	return res
}

func ParseAddr(addr string) (block.AddressType, error) {
	var ra block.AddressType
	buf, err := base58.Decode(addr)
	if err != nil {
		return ra, err
	}
	if len(buf) != block.AddressLen+2 {
		return ra, errors.New("addr len invalid")
	}
	if buf[0] != 1 {
		return ra, errors.New("addr type invalid")
	}
	copy(ra[:], buf[1:1+block.AddressLen])
	if buf[1+block.AddressLen] != checkSum(ra) {
		return ra, errors.New("addr checksum invalid")
	}
	return ra, nil
}

func EncodeAddr(addr block.AddressType) string {
	buf := make([]byte, block.AddressLen+2)
	buf[0] = 1
	copy(buf[1:1+block.AddressLen], addr[:])
	buf[1+block.AddressLen] = checkSum(addr)
	return "tcoin" + base58.Encode(buf)
}
