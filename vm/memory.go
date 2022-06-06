package vm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"
)

// todo: add write privilege check

var ErrInvalidELF = errors.New("invalid ELF")

const PageSize = 4096
const MaxPagesPerBlock = 256
const NumBlocks = 5
const OpRead = 4
const OpWrite = 2
const OpExecute = 1

type Page [PageSize >> 3]uint64

var pagePool = &sync.Pool{New: func() interface{} {
	return &Page{}
}}

type Pages [MaxPagesPerBlock]*Page

type ProgramMemory struct {
	blocks [NumBlocks]Pages
}

type Memory struct {
	programs map[uint32]*ProgramMemory
}

func (ps *Pages) check(id uint32) bool {
	return ps[id] != nil
}

func (ps *Pages) assure(id uint32) bool {
	if ps[id] == nil {
		p := pagePool.Get().(*Page)
		ps[id] = p
		p[0] = 0
		for i := 1; i < (PageSize >> 4); i <<= 1 {
			copy(p[i:i+i], p[:i])
		}
		return true
	}
	return false
}

func (ps *Pages) recycle() {
	for _, x := range ps {
		if x != nil {
			pagePool.Put(x)
			x = nil
		}
	}
}

func (pm *ProgramMemory) Access(ptr uint32, isSelf bool, op int) (*uint64, bool) {
	bid := ptr >> 28
	pageId := (ptr << 4) >> 16
	pagePos := (ptr >> 3) & 0x1ff
	if bid == 0 || bid >= 6 || pageId >= MaxPagesPerBlock {
		return nil, false
	}
	if isSelf {
		if op == OpWrite {
			if bid == 1 {
				return nil, false
			}
		} else if op == OpExecute {
			if bid != 1 {
				return nil, false
			}
		}
	} else {
		if op == OpWrite {
			return nil, false
		} else {
			if bid != 4 && bid != 5 {
				return nil, false
			}
		}
	}
	allocated := false
	if bid == 1 || !isSelf {
		if !pm.blocks[bid-1].check(pageId) {
			return nil, false
		}
	} else {
		allocated = pm.blocks[bid-1].assure(pageId)
	}
	return &pm.blocks[bid-1][pageId][pagePos], allocated
}

func (pm *ProgramMemory) Recycle() {
	for i := 0; i < NumBlocks; i++ {
		pm.blocks[i].recycle()
	}
}

type segment struct {
	offset   uint32
	length   uint32
	addr     uint32
	numPages uint32
}

func (pm *ProgramMemory) LoadELF(elf []byte, loadOffset uint32) (uint32, error) {
	const SizeLimit = 1 << 30
	if len(elf) < 0x40 {
		return 0, ErrInvalidELF
	}
	if binary.LittleEndian.Uint32(elf[:4]) != 0x464c457f {
		return 0, ErrInvalidELF
	}
	if binary.LittleEndian.Uint16(elf[0x12:0x14]) != 243 {
		return 0, ErrInvalidELF
	}
	entryPoint := binary.LittleEndian.Uint64(elf[0x18:0x20])
	if entryPoint >= (1 << 32) {
		return 0, ErrInvalidELF
	}
	programHeaderOffset := int(binary.LittleEndian.Uint64(elf[0x20:0x28]))
	if programHeaderOffset > SizeLimit || programHeaderOffset < 0 {
		return 0, ErrInvalidELF
	}
	programHeaderEntrySize := int(binary.LittleEndian.Uint16(elf[0x36:0x38]))
	numProgramHeaderEntries := int(binary.LittleEndian.Uint16(elf[0x38:0x3a]))
	if len(elf) < programHeaderOffset+programHeaderEntrySize*numProgramHeaderEntries {
		return 0, ErrInvalidELF
	}
	if programHeaderEntrySize != 56 {
		return 0, ErrInvalidELF
	}
	segments := []segment{}
	for i := 0; i < numProgramHeaderEntries; i++ {
		entry := elf[programHeaderOffset+programHeaderEntrySize*i : programHeaderOffset+programHeaderEntrySize*(i+1)]
		p_type := binary.LittleEndian.Uint32(entry[:4])
		p_flags := binary.LittleEndian.Uint32(entry[4:8])
		p_offset := binary.LittleEndian.Uint64(entry[8:16])
		p_vaddr := binary.LittleEndian.Uint64(entry[16:24])
		p_filesz := binary.LittleEndian.Uint64(entry[32:40])
		p_memsz := binary.LittleEndian.Uint64(entry[40:48])
		p_align := binary.LittleEndian.Uint64(entry[48:56])
		if p_type != 1 {
			return 0, ErrInvalidELF
		}
		if p_align != PageSize {
			return 0, ErrInvalidELF
		}
		privileges := p_flags & 7
		if len(elf) < int(p_offset+p_filesz) || p_offset+p_filesz > SizeLimit {
			return 0, ErrInvalidELF
		}
		if p_vaddr%PageSize != 0 || p_memsz%PageSize != 0 || p_memsz > SizeLimit {
			return 0, ErrInvalidELF
		}
		numPages := p_memsz / PageSize
		if numPages > MaxPagesPerBlock {
			return 0, ErrInvalidELF
		}
		if p_vaddr >= (1 << 32) {
			return 0, ErrInvalidELF
		}
		tmp1 := p_vaddr + uint64(loadOffset)
		tmp2 := tmp1 + p_memsz
		bid := tmp1 >> 28
		if tmp2 >= (1<<32) || bid != (tmp2>>28) || bid < 1 || bid > NumBlocks {
			return 0, ErrInvalidELF
		}
		addr := uint32(tmp1)
		for i := 0; i < int(numPages); i++ {
			pAddr := addr + uint32(i)*PageSize
			pageId := (pAddr << 4) >> 16
			if pageId >= MaxPagesPerBlock {
				return 0, ErrInvalidELF
			}
			var page *Page = pm.blocks[bid-1][pageId]
			if page != nil {
				return 0, ErrInvalidELF
			}
		}
		if bid == 1 {
			if (privileges & 5) != privileges {
				return 0, ErrInvalidELF
			}
		} else {
			if (privileges & 6) != privileges {
				return 0, ErrInvalidELF
			}
		}
		segments = append(segments, segment{
			offset:   uint32(p_offset),
			length:   uint32(p_filesz),
			addr:     addr,
			numPages: uint32(numPages),
		})
	}
	for _, segment := range segments {
		bid := segment.addr >> 28
		for i := 0; i < int(segment.numPages); i++ {
			pm.Access(segment.addr+uint32(i*PageSize), true, OpRead)
		}
		buf := bytes.NewBuffer(elf[segment.offset : segment.offset+segment.length])
		for i := 0; i < int(segment.numPages); i++ {
			pAddr := segment.addr + uint32(i)*PageSize
			pageId := (pAddr << 4) >> 16
			start := i * PageSize
			end := (i + 1) * PageSize
			if end <= int(segment.length) {
				binary.Read(buf, binary.LittleEndian, pm.blocks[bid-1][pageId])
			} else if start < int(segment.length) {
				rem := segment.length % PageSize
				rem8 := rem >> 3
				binary.Read(buf, binary.LittleEndian, pm.blocks[bid-1][pageId][:rem8])
				if (rem & 7) != 0 {
					buf2 := make([]byte, 8)
					copy(buf2[:rem&7], buf.Bytes())
					pm.blocks[bid-1][pageId][rem8] = binary.LittleEndian.Uint64(buf2)
				}
			}
		}
	}
	return uint32(entryPoint), nil
}

// todo: test LoadELF
