package consensus

import (
	"bytes"
	"encoding/binary"
	"io"
	"math/big"

	"github.com/mcfx/tcoin/core/block"
)

type ConsensusState struct {
	Height           int
	LastBlockTime    uint64
	LastKeyBlockTime uint64
	Difficulty       block.HashType
}

const PeriodBlockCount = 30
const PeriodTime = 300 // 10s per block

func (cs *ConsensusState) CheckAndUpdate(blk *block.Block) bool {
	if bytes.Compare(blk.Header.Hash[:], cs.Difficulty[:]) > 0 {
		return false
	}
	if blk.Time <= cs.LastBlockTime {
		return false
	}
	cs.Height++
	cs.LastBlockTime = blk.Time
	if cs.Height%PeriodBlockCount == 0 {
		dt := big.NewInt(0)
		dt.SetBytes(cs.Difficulty[:])
		rtime := blk.Time - cs.LastKeyBlockTime
		wtime := uint64(PeriodTime * 1000000000)
		rmin := wtime * 15 / 16
		rmax := wtime * 17 / 16
		if rtime < rmin {
			rtime = rmin
		} else if rtime > rmax {
			rtime = rmax
		}
		dt.Mul(dt, big.NewInt(int64(rtime)))
		dt.Div(dt, big.NewInt(int64(wtime)))
		if dt.BitLen() > 256 {
			cs.Difficulty[0] = 0xff
		} else {
			dt.FillBytes(cs.Difficulty[:])
		}
		cs.LastKeyBlockTime = blk.Time
	}
	return true
}

func DecodeConsensus(r io.Reader) (*ConsensusState, error) {
	cs := &ConsensusState{}
	buf := make([]byte, 8*3+block.HashLen)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	cs.Height = int(binary.LittleEndian.Uint64(buf[:8]))
	cs.LastBlockTime = binary.LittleEndian.Uint64(buf[8:16])
	cs.LastKeyBlockTime = binary.LittleEndian.Uint64(buf[16:24])
	copy(cs.Difficulty[:], buf[24:])
	return cs, nil
}

func EncodeConsensus(w io.Writer, cs *ConsensusState) error {
	buf := make([]byte, 8*3+block.HashLen)
	binary.LittleEndian.PutUint64(buf[:8], uint64(cs.Height))
	binary.LittleEndian.PutUint64(buf[8:16], cs.LastBlockTime)
	binary.LittleEndian.PutUint64(buf[16:24], cs.LastKeyBlockTime)
	copy(buf[24:], cs.Difficulty[:])
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return nil
}
