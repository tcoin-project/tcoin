package block

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"

	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/utils"
)

type BlockHeader struct {
	Hash       HashType `json:"hash"`
	ParentHash HashType `json:"parent_hash"`
	BodyHash   HashType `json:"body_hash"`
	ExtraData  HashType `json:"extra_data"`
}

type Block struct {
	Header BlockHeader    `json:"header"`
	Miner  AddressType    `json:"miner"`
	Time   uint64         `json:"time"`
	Txs    []*Transaction `json:"txs"`
}

func (bh *BlockHeader) ComputeHash() HashType {
	buf := make([]byte, HashLen*3)
	copy(buf[:HashLen], bh.ParentHash[:])
	copy(buf[HashLen:HashLen*2], bh.BodyHash[:])
	copy(buf[HashLen*2:HashLen*3], bh.ExtraData[:])
	return sha256.Sum256(buf)
}

func DecodeBlockHeader(r utils.Reader) (BlockHeader, error) {
	var bh BlockHeader
	buf := make([]byte, HashLen*4)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return bh, err
	}
	copy(bh.Hash[:], buf[:HashLen])
	copy(bh.ParentHash[:], buf[HashLen:HashLen*2])
	copy(bh.BodyHash[:], buf[HashLen*2:HashLen*3])
	copy(bh.ExtraData[:], buf[HashLen*3:HashLen*4])
	if sha256.Sum256(buf[HashLen:]) != bh.Hash {
		return bh, errors.New("block header hash mismatch")
	}
	return bh, nil
}

func EncodeBlockHeader(w utils.Writer, bh BlockHeader) error {
	buf := make([]byte, HashLen*4)
	copy(buf[:HashLen], bh.Hash[:])
	copy(buf[HashLen:HashLen*2], bh.ParentHash[:])
	copy(buf[HashLen*2:HashLen*3], bh.BodyHash[:])
	copy(buf[HashLen*3:HashLen*4], bh.ExtraData[:])
	_, err := w.Write(buf)
	if err != nil {
		return err
	}
	return nil
}

func (b *Block) ComputeHash() HashType {
	var buf bytes.Buffer
	EncodeBlockSelf(&buf, b)
	return sha256.Sum256(buf.Bytes())
}

func (b *Block) FillHash() {
	b.Header.BodyHash = b.ComputeHash()
	b.Header.Hash = b.Header.ComputeHash()
}

func DecodeBlock(r utils.Reader) (*Block, error) {
	bh, err := DecodeBlockHeader(r)
	if err != nil {
		return nil, err
	}
	b := &Block{Header: bh}
	_, err = io.ReadFull(r, b.Miner[:])
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 8)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, err
	}
	b.Time = binary.LittleEndian.Uint64(buf)
	txCount, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}
	if txCount > (1 << 20) {
		return nil, errors.New("too much transactions")
	}
	b.Txs = make([]*Transaction, txCount)
	for i := 0; i < int(txCount); i++ {
		b.Txs[i], err = DecodeTx(r)
		if err != nil {
			return nil, err
		}
	}
	if b.ComputeHash() != b.Header.BodyHash {
		return nil, errors.New("block body hash mismatch")
	}
	return b, nil
}

func EncodeBlockSelf(w utils.Writer, b *Block) error {
	_, err := w.Write(b.Miner[:])
	if err != nil {
		return err
	}
	buf := make([]byte, 8+binary.MaxVarintLen64)
	binary.LittleEndian.PutUint64(buf[:8], b.Time)
	cur := binary.PutUvarint(buf[8:], uint64(len(b.Txs))) + 8
	_, err = w.Write(buf[:cur])
	if err != nil {
		return err
	}
	for _, tx := range b.Txs {
		err = EncodeTx(w, tx)
		if err != nil {
			return err
		}
	}
	return nil
}

func EncodeBlock(w utils.Writer, b *Block) error {
	err := EncodeBlockHeader(w, b.Header)
	if err != nil {
		return err
	}
	err = EncodeBlockSelf(w, b)
	if err != nil {
		return err
	}
	return nil
}

func ExecuteBlock(b *Block, reward uint64, s *storage.Slice, ctx *ExecutionContext) error {
	var totalFee uint64 = 0
	for _, tx := range b.Txs {
		err := ExecuteTx(tx, s, ctx)
		if err != nil {
			return err
		}
		totalFee += tx.Fee // it won't overflow since the total amount is bounded
	}
	info := GetAccountInfo(s, b.Miner)
	info.Balance += totalFee + reward
	SetAccountInfo(s, b.Miner, info)
	if ctx.Callback != nil {
		ctx.Callback.Transfer(s, AddressType{}, b.Miner, totalFee+reward, nil, ctx)
	}
	return nil
}
