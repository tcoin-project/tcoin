package network

import (
	"encoding/binary"
	"errors"
	"io"
)

type packet struct {
	tp   byte
	data []byte
}

func decodePacket(r io.Reader) (packet, error) {
	var p packet
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return p, err
	}
	x := int(binary.LittleEndian.Uint32(buf))
	p.tp = byte(x >> 24)
	x &= 0xffffff
	p.data = make([]byte, x)
	if _, err := io.ReadFull(r, p.data); err != nil {
		return packet{}, err
	}
	return p, nil
}

func encodePacket(w io.Writer, p packet) error {
	if len(p.data) > 0xffffff {
		return errors.New("packet too large")
	}
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(len(p.data)))
	buf[3] = p.tp
	if _, err := w.Write(buf); err != nil {
		return err
	}
	if _, err := w.Write(p.data); err != nil {
		return err
	}
	return nil
}
