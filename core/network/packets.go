package network

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/storage"
)

type PacketBlockRequest struct {
	MinId int
	Hash  [storage.SliceKeyLen]byte
}

type PacketBlocks struct {
	MinId  int
	isFull []byte
	Body   []interface{}
}

type PacketTransactions struct {
	Txs []*block.Transaction
}

func DecodeBlockRequest(r *bytes.Buffer) (PacketBlockRequest, error) {
	p := PacketBlockRequest{}
	t, err := binary.ReadUvarint(r)
	if err != nil {
		return p, err
	}
	p.MinId = int(t)
	_, err = io.ReadFull(r, p.Hash[:])
	if err != nil {
		return p, err
	}
	return p, nil
}

func EncodeBlockRequest(w *bytes.Buffer, p PacketBlockRequest) error {
	buf := make([]byte, binary.MaxVarintLen64)
	n := binary.PutUvarint(buf, uint64(p.MinId))
	w.Write(buf[:n])
	w.Write(p.Hash[:])
	return nil
}

func (p *PacketBlocks) IsFull(id int) bool {
	return (p.isFull[id>>3] >> (id & 7) & 1) == 1
}

func (p *PacketBlocks) Add(b interface{}, isFull bool) {
	pos := len(p.Body)
	if pos%8 == 0 {
		p.isFull = append(p.isFull, 0)
	}
	if isFull {
		p.isFull[pos>>3] |= byte(1 << (pos & 7))
	}
	p.Body = append(p.Body, b)
}

func NewPakcetBlocks(minId int) PacketBlocks {
	return PacketBlocks{
		MinId:  minId,
		isFull: make([]byte, 0),
		Body:   make([]interface{}, 0),
	}
}

func DecodeBlocks(r *bytes.Buffer) (PacketBlocks, error) {
	p := PacketBlocks{}
	t, err := binary.ReadUvarint(r)
	if err != nil {
		return p, err
	}
	p.MinId = int(t)
	t, err = binary.ReadUvarint(r)
	if err != nil {
		return p, err
	}
	cnt := int(t)
	if cnt < 0 {
		return p, errors.New("negative block count")
	}
	if cnt > 114514 {
		return p, errors.New("too many blocks")
	}
	cnt8 := (cnt + 7) / 8
	p.isFull = make([]byte, cnt8)
	_, err = io.ReadFull(r, p.isFull)
	if err != nil {
		return p, err
	}
	for i := 0; i < cnt; i++ {
		var blk interface{}
		if p.IsFull(i) {
			blk, err = block.DecodeBlock(r)
		} else {
			blk, err = block.DecodeBlockHeader(r)
		}
		if err != nil {
			return PacketBlocks{}, err
		}
		p.Body = append(p.Body, blk)
	}
	return p, nil
}

func EncodeBlocks(w *bytes.Buffer, p PacketBlocks) error {
	buf := make([]byte, binary.MaxVarintLen64*2)
	cur := binary.PutUvarint(buf, uint64(p.MinId))
	cur = binary.PutUvarint(buf[cur:], uint64(len(p.Body))) + cur
	_, err := w.Write(buf[:cur])
	if err != nil {
		return err
	}
	_, err = w.Write(p.isFull)
	if err != nil {
		return err
	}
	for i, b := range p.Body {
		if p.IsFull(i) {
			err = block.EncodeBlock(w, b.(*block.Block))
		} else {
			err = block.EncodeBlockHeader(w, b.(block.BlockHeader))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func DecodeTransactions(r *bytes.Buffer) (PacketTransactions, error) {
	p := PacketTransactions{
		Txs: make([]*block.Transaction, 0),
	}
	for r.Len() != 0 {
		tx, err := block.DecodeTx(r)
		if err != nil {
			return PacketTransactions{}, err
		}
		p.Txs = append(p.Txs, tx)
	}
	return p, nil
}

func EncodeTransactions(w *bytes.Buffer, p PacketTransactions) error {
	for _, tx := range p.Txs {
		err := block.EncodeTx(w, tx)
		if err != nil {
			return err
		}
	}
	return nil
}
