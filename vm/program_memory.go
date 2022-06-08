package vm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

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

var pmPool = &sync.Pool{New: func() interface{} {
	return &ProgramMemory{}
}}

func (ps *Pages) check(id uint32) bool {
	return ps[id] != nil
}

func (ps *Pages) assure(id uint32) bool {
	if ps[id] == nil {
		p := pagePool.Get().(*Page)
		ps[id] = p
		p[0] = 0
		for i := 1; i <= (PageSize >> 4); i <<= 1 {
			copy(p[i:i+i], p[:i])
		}
		return true
	}
	return false
}

func (ps *Pages) recycle() {
	for i := 0; i < MaxPagesPerBlock; i++ {
		if ps[i] != nil {
			pagePool.Put(ps[i])
			ps[i] = nil
		}
	}
}

func (pm *ProgramMemory) Access(ptr uint32, isSelf bool, op int) (*uint64, bool) {
	if !isSelf && op == OpExecute {
		panic("can't execute when it's not self")
	}
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
			if bid != 1 && bid != 4 && bid != 5 {
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

func (pm *ProgramMemory) load(addr, numPages uint32, s []byte) {
	length := len(s)
	bid := addr >> 28
	for i := 0; i < int(numPages); i++ {
		pm.Access(addr+uint32(i*PageSize), true, OpRead)
	}
	buf := bytes.NewBuffer(s)
	for i := 0; i < int(numPages); i++ {
		pAddr := addr + uint32(i)*PageSize
		pageId := (pAddr << 4) >> 16
		start := i * PageSize
		end := (i + 1) * PageSize
		pm.blocks[bid-1].assure(pageId)
		if end <= length {
			binary.Read(buf, binary.LittleEndian, pm.blocks[bid-1][pageId])
		} else if start < length {
			rem := length % PageSize
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

func (pm *ProgramMemory) LoadRawCode(code []byte, loadOffset uint32, env *ExecEnv) error {
	if len(code) >= (1<<32) || len(code)+int(loadOffset) >= (1<<32) {
		return errors.New("code too long")
	}
	if loadOffset%PageSize != 0 {
		return fmt.Errorf("load offset %d not aligned", loadOffset)
	}
	bid := loadOffset >> 28
	if bid != 1 {
		return fmt.Errorf("can only load to block 1")
	}
	numPages := (len(code) + PageSize - 1) / PageSize
	for i := 0; i < int(numPages); i++ {
		pAddr := loadOffset + uint32(i)*PageSize
		pageId := (pAddr << 4) >> 16
		if pageId >= MaxPagesPerBlock {
			return fmt.Errorf("page id %d too large", pageId)
		}
		var page *Page = pm.blocks[bid-1][pageId]
		if page != nil {
			return fmt.Errorf("page %d already allocated", pageId)
		}
	}
	if env.Gas < uint64(numPages)*GasMemoryPage {
		return ErrInsufficientGas
	}
	env.Gas -= uint64(numPages) * GasMemoryPage
	pm.load(loadOffset, uint32(numPages), code)
	return nil
}

func (pm *ProgramMemory) LoadELF(elf []byte, loadOffset uint32, env *ExecEnv) (uint32, error) {
	const SizeLimit = 1 << 30
	if loadOffset%PageSize != 0 {
		return 0, fmt.Errorf("load offset %d not aligned", loadOffset)
	}
	if len(elf) < 0x40 {
		return 0, fmt.Errorf("invalid ELF: size too small (%d < 0x40)", len(elf))
	}
	if binary.LittleEndian.Uint32(elf[:4]) != 0x464c457f {
		return 0, fmt.Errorf("invalid ELF: header magic mismatch")
	}
	if binary.LittleEndian.Uint16(elf[0x12:0x14]) != 243 {
		return 0, fmt.Errorf("invalid ELF: arch mismatch")
	}
	entryPoint := binary.LittleEndian.Uint64(elf[0x18:0x20])
	if entryPoint >= (1<<32) || entryPoint+uint64(loadOffset) >= (1<<32) {
		return 0, fmt.Errorf("invalid ELF: entry point too large: %d", entryPoint)
	}
	programHeaderOffset := int(binary.LittleEndian.Uint64(elf[0x20:0x28]))
	if programHeaderOffset > SizeLimit || programHeaderOffset < 0 {
		return 0, fmt.Errorf("invalid ELF: program header offset invalid: %d", programHeaderOffset)
	}
	programHeaderEntrySize := int(binary.LittleEndian.Uint16(elf[0x36:0x38]))
	numProgramHeaderEntries := int(binary.LittleEndian.Uint16(elf[0x38:0x3a]))
	targetLen := programHeaderOffset + programHeaderEntrySize*numProgramHeaderEntries
	if len(elf) < targetLen {
		return 0, fmt.Errorf("invalid ELF: size too small (%d < %d)", len(elf), targetLen)
	}
	if programHeaderEntrySize != 56 {
		return 0, fmt.Errorf("invalid ELF: program header entry size mismatch (%d != 56)", programHeaderEntrySize)
	}
	segments := []segment{}
	totPages := 0
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
			return 0, fmt.Errorf("invalid ELF: segment type %d unsupported", p_type)
		}
		if p_align != PageSize {
			return 0, fmt.Errorf("invalid ELF: align %d unsupported", p_align)
		}
		privileges := p_flags & 7
		if p_offset+p_filesz > SizeLimit {
			return 0, fmt.Errorf("invalid ELF: offset too large: %d", p_offset+p_filesz)
		}
		if len(elf) < int(p_offset+p_filesz) {
			return 0, fmt.Errorf("invalid ELF: size too small (%d < %d)", len(elf), p_offset+p_filesz)
		}
		if p_memsz > SizeLimit {
			return 0, fmt.Errorf("invalid ELF: memsz too large: %d", p_memsz)
		}
		if p_vaddr%PageSize != 0 {
			return 0, fmt.Errorf("invalid ELF: vaddr not aligned: %d", p_vaddr)
		}
		numPages := (p_memsz + PageSize - 1) / PageSize
		if numPages > MaxPagesPerBlock {
			return 0, fmt.Errorf("invalid ELF: too many pages: %d", numPages)
		}
		if p_vaddr >= (1 << 32) {
			return 0, fmt.Errorf("invalid ELF: vaddr too large: %d", p_vaddr)
		}
		tmp1 := p_vaddr + uint64(loadOffset)
		tmp2 := tmp1 + numPages*PageSize
		bid := tmp1 >> 28
		if tmp2 >= (1<<32) || bid != (tmp2>>28) {
			return 0, fmt.Errorf("invalid ELF: segment not in one memory block (end=%d)", tmp2)
		}
		if bid < 1 || bid > NumBlocks {
			return 0, fmt.Errorf("invalid ELF: unknown block id %d", bid)
		}
		addr := uint32(tmp1)
		for i := 0; i < int(numPages); i++ {
			pAddr := addr + uint32(i)*PageSize
			pageId := (pAddr << 4) >> 16
			if pageId >= MaxPagesPerBlock {
				return 0, fmt.Errorf("invalid ELF: page id %d too large", pageId)
			}
			var page *Page = pm.blocks[bid-1][pageId]
			if page != nil {
				return 0, fmt.Errorf("invalid ELF: page %d already allocated", pageId)
			}
		}
		if bid == 1 {
			if (privileges & 5) != privileges {
				return 0, fmt.Errorf("invalid ELF: block 1 only supports rx (required %d)", privileges)
			}
		} else {
			if (privileges & 6) != privileges {
				return 0, fmt.Errorf("invalid ELF: block %d only supports rw (required %d)", bid, privileges)
			}
		}
		segments = append(segments, segment{
			offset:   uint32(p_offset),
			length:   uint32(p_filesz),
			addr:     addr,
			numPages: uint32(numPages),
		})
		totPages += int(numPages)
	}
	if env.Gas < uint64(totPages)*GasMemoryPage {
		return 0, ErrInsufficientGas
	}
	env.Gas -= uint64(totPages) * GasMemoryPage
	for _, segment := range segments {
		pm.load(segment.addr, segment.numPages, elf[segment.offset:segment.offset+segment.length])
	}
	return uint32(entryPoint) + loadOffset, nil
}
