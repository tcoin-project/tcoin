package core

import (
	"encoding/binary"
	"encoding/hex"
	"errors"

	"github.com/mcfx/tcoin/core/block"
	"github.com/mcfx/tcoin/storage"
	"github.com/mcfx/tcoin/utils/address"
)

func explorerListAdd(s *storage.Slice, k storage.KeyType, v storage.DataType) {
	t := s.Read(k)
	x := binary.BigEndian.Uint64(t[:8])
	binary.BigEndian.PutUint64(t[:8], x+1)
	s.Write(k, t)
	copy(k[storage.KeyLen-8:], t[:8])
	s.Write(k, v)
}

func explorerListAddAddr(s *storage.Slice, addr block.AddressType, id uint64, v storage.DataType) {
	k := storage.KeyType{0xf0}
	copy(k[1:1+block.AddressLen], addr[:])
	binary.LittleEndian.PutUint64(k[1+block.AddressLen:9+block.AddressLen], id)
	explorerListAdd(s, k, v)
}

func explorerReadList(s *storage.Slice, k storage.KeyType, l int, r int) ([]storage.DataType, int) {
	t := s.Read(k)
	x := binary.BigEndian.Uint64(t[:8])
	res := make([]storage.DataType, 0)
	// return latest l to r
	for i := int(x) - l + 1; i >= int(x)-r+1 && i >= 1; i-- {
		binary.BigEndian.PutUint64(k[storage.KeyLen-8:], uint64(i))
		res = append(res, s.Read(k))
	}
	return res, int(x)
}

func explorerReadListAddr(s *storage.Slice, addr block.AddressType, id uint64, l int, r int) ([]storage.DataType, int) {
	k := storage.KeyType{0xf0}
	copy(k[1:1+block.AddressLen], addr[:])
	binary.LittleEndian.PutUint64(k[1+block.AddressLen:9+block.AddressLen], id)
	return explorerReadList(s, k, l, r)
}

func explorerTransferCallback(s *storage.Slice, from block.AddressType, to block.AddressType, value uint64, tx *block.Transaction, ctx *block.ExecutionContext) {
	v := storage.DataType{}
	binary.LittleEndian.PutUint64(v[:8], value)
	binary.LittleEndian.PutUint64(v[8:16], ctx.Time)
	binary.LittleEndian.PutUint64(v[16:24], uint64(ctx.Height))
	var txh block.HashType
	if tx != nil {
		txh = tx.Hash()
	}
	if from != (block.AddressType{}) {
		explorerListAddAddr(s, from, 101, storage.DataType(from))
		explorerListAddAddr(s, from, 102, storage.DataType(to))
		explorerListAddAddr(s, from, 103, storage.DataType(txh))
		explorerListAddAddr(s, from, 104, v)
	}
	if from != to {
		explorerListAddAddr(s, to, 101, storage.DataType(from))
		explorerListAddAddr(s, to, 102, storage.DataType(to))
		explorerListAddAddr(s, to, 103, storage.DataType(txh))
		explorerListAddAddr(s, to, 104, v)
	}
	if tx != nil {
		k := storage.KeyType{0xf1}
		copy(k[1:1+block.HashLen], txh[:])
		v2 := storage.DataType{}
		binary.LittleEndian.PutUint64(v2[:8], uint64(ctx.Height))
		s.Write(k, v2)
	}
}

func ExplorerExecutionCallback() *block.ExecutionCallback {
	return &block.ExecutionCallback{
		Transfer: explorerTransferCallback,
	}
}

type ExplorerTransaction struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Hash    string `json:"hash"`
	Value   uint64 `json:"value"`
	BlockId int    `json:"blockid"`
	Time    int    `json:"time"`
}

func (cn *ChainNode) ExplorerGetAccountTransactions(addr block.AddressType, l int, r int) ([]ExplorerTransaction, int) {
	cn.seMut.Lock()
	s := cn.se.HighestSlice
	froms, n := explorerReadListAddr(s, addr, 101, l, r)
	tos, _ := explorerReadListAddr(s, addr, 102, l, r)
	txh, _ := explorerReadListAddr(s, addr, 103, l, r)
	vs, _ := explorerReadListAddr(s, addr, 104, l, r)
	cn.seMut.Unlock()
	res := make([]ExplorerTransaction, len(froms))
	for i := 0; i < len(froms); i++ {
		res[i].From = address.EncodeAddr(block.AddressType(froms[i]))
		res[i].To = address.EncodeAddr(block.AddressType(tos[i]))
		res[i].Hash = hex.EncodeToString(txh[i][:])
		res[i].Value = binary.LittleEndian.Uint64(vs[i][:8])
		res[i].Time = int(binary.LittleEndian.Uint64(vs[i][8:16]))
		res[i].BlockId = int(binary.LittleEndian.Uint64(vs[i][16:24]))
	}
	return res, n
}

func (cn *ChainNode) ExplorerGetTransaction(txh block.HashType) (*block.Transaction, int, error) {
	cn.seMut.Lock()
	s := cn.se.HighestSlice
	k := storage.KeyType{0xf1}
	copy(k[1:1+block.HashLen], txh[:])
	v := s.Read(k)
	hc := cn.se.HighestChain
	cn.seMut.Unlock()
	height := int(binary.LittleEndian.Uint64(v[:8]))
	mh := hc[len(hc)-1].S.Height()
	hash := block.HashType{}
	if height >= mh {
		hash = block.HashType(hc[height-hc[0].S.Height()].Key)
	}
	b, err := cn.getBlock(height, hash)
	if err != nil {
		return nil, 0, err
	}
	for _, tx := range b.Txs {
		if tx.Hash() == txh {
			return tx, height, nil
		}
	}
	return nil, 0, errors.New("transaction not found")
}
