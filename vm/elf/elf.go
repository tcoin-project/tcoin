package elf

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const SizeLimit = 1 << 30
const PageSize = 1 << 12

type Segment struct {
	Privileges uint8
	Offset     uint32
	Addr       uint32
	FileSz     uint32
	MemSz      uint32
}

type ELF struct {
	Entry               uint32
	ProgramHeaderOffset uint32
	Segments            []Segment
}

func ParseELF(elf []byte) (*ELF, error) {
	r := &ELF{
		Segments: []Segment{},
	}

	if len(elf) < 0x40 {
		return nil, fmt.Errorf("size too small (%d < 0x40)", len(elf))
	}
	if len(elf) > SizeLimit {
		return nil, fmt.Errorf("size too large (%d)", len(elf))
	}
	if binary.LittleEndian.Uint32(elf[:4]) != 0x464c457f {
		return nil, errors.New("header magic mismatch")
	}
	if binary.LittleEndian.Uint16(elf[0x12:0x14]) != 243 {
		return nil, errors.New("arch mismatch")
	}
	entryPoint := binary.LittleEndian.Uint64(elf[0x18:0x20])
	if entryPoint >= SizeLimit {
		return nil, fmt.Errorf("entry point too large: %d", entryPoint)
	}
	r.Entry = uint32(entryPoint)
	programHeaderOffset := int(binary.LittleEndian.Uint64(elf[0x20:0x28]))
	if programHeaderOffset > SizeLimit || programHeaderOffset < 0 {
		return nil, fmt.Errorf("program header offset invalid: %d", programHeaderOffset)
	}
	r.ProgramHeaderOffset = uint32(programHeaderOffset)
	programHeaderEntrySize := int(binary.LittleEndian.Uint16(elf[0x36:0x38]))
	numProgramHeaderEntries := int(binary.LittleEndian.Uint16(elf[0x38:0x3a]))
	targetLen := programHeaderOffset + programHeaderEntrySize*numProgramHeaderEntries
	if len(elf) < targetLen {
		return nil, fmt.Errorf("size too small (%d < %d)", len(elf), targetLen)
	}
	if programHeaderEntrySize != 56 {
		return nil, fmt.Errorf("invalid ELF: program header entry size invalid (%d != 56)", programHeaderEntrySize)
	}
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
			return nil, fmt.Errorf("segment type %d unsupported", p_type)
		}
		if p_align != PageSize {
			return nil, fmt.Errorf("align %d unsupported", p_align)
		}
		privileges := p_flags & 7
		if p_offset+p_filesz > SizeLimit {
			return nil, fmt.Errorf("offset too large: %d", p_offset+p_filesz)
		}
		if len(elf) < int(p_offset+p_filesz) {
			return nil, fmt.Errorf("size too small (%d < %d)", len(elf), p_offset+p_filesz)
		}
		if p_vaddr > SizeLimit {
			return nil, fmt.Errorf("vaddr too large: %d", p_memsz)
		}
		if p_memsz > SizeLimit {
			return nil, fmt.Errorf("memsz too large: %d", p_memsz)
		}
		if p_memsz < p_filesz {
			return nil, fmt.Errorf("memsz smaller than filesz: %d < %d", p_memsz, p_filesz)
		}
		if p_vaddr%PageSize != 0 {
			return nil, fmt.Errorf("vaddr not aligned: %d", p_vaddr)
		}
		if p_offset%8 != 0 {
			return nil, fmt.Errorf("offset not aligned: %d", p_offset)
		}
		r.Segments = append(r.Segments, Segment{
			Privileges: uint8(privileges),
			Offset:     uint32(p_offset),
			Addr:       uint32(p_vaddr),
			FileSz:     uint32(p_filesz),
			MemSz:      uint32(p_memsz),
		})
	}
	return r, nil
}

func trimZeros(s []byte, minSz int) []byte {
	cur := len(s) - 1
	for cur >= minSz && s[cur] == 0 {
		cur--
	}
	return s[:cur+1]
}

func TrimELF(s []byte, e *ELF, ignoreSegs []uint32, newEntry uint64) ([]byte, error) {
	seg0ReqSz := e.ProgramHeaderOffset + 56*uint32(len(e.Segments))
	if e.Segments[0].Offset != 0 || e.Segments[0].FileSz < seg0ReqSz {
		return nil, errors.New("unsupported first segment")
	}
	ign := map[uint32]bool{}
	for _, x := range ignoreSegs {
		ign[x] = true
	}
	cur := e.ProgramHeaderOffset
	n := 0
	resf := []byte{}
	for i, seg := range e.Segments {
		if _, ok := ign[seg.Addr]; ok {
			if i == 0 {
				return nil, errors.New("can't trim first segment")
			}
			continue
		}
		reqSz := 0
		if i == 0 {
			reqSz = int(seg0ReqSz)
		}
		nseg := trimZeros(s[seg.Offset:seg.Offset+seg.FileSz], reqSz)
		olen := len(resf)
		resf = append(resf, nseg...)
		for (len(resf) & 7) != 0 {
			resf = append(resf, 0)
		}
		binary.LittleEndian.PutUint32(resf[cur:cur+4], 1)
		binary.LittleEndian.PutUint32(resf[cur+4:cur+8], uint32(seg.Privileges))
		binary.LittleEndian.PutUint64(resf[cur+8:cur+16], uint64(olen))
		binary.LittleEndian.PutUint64(resf[cur+16:cur+24], uint64(seg.Addr))
		binary.LittleEndian.PutUint64(resf[cur+24:cur+32], uint64(seg.Addr))
		binary.LittleEndian.PutUint64(resf[cur+32:cur+40], uint64(len(nseg)))
		binary.LittleEndian.PutUint64(resf[cur+40:cur+48], uint64(seg.MemSz))
		binary.LittleEndian.PutUint64(resf[cur+48:cur+56], PageSize)
		cur += 56
		n++
	}
	binary.LittleEndian.PutUint16(resf[0x38:0x3a], uint16(n))
	if cur < seg0ReqSz {
		zeros := make([]byte, 56)
		for ; cur < seg0ReqSz; cur += 56 {
			copy(resf[cur:cur+56], zeros)
		}
	}
	binary.LittleEndian.PutUint64(resf[0x18:0x20], newEntry)
	return resf, nil
}
