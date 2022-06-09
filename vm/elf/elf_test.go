package elf

import (
	"bytes"
	"io/ioutil"
	"os/exec"
	"reflect"
	"testing"
)

func getTestELF() []byte {
	code := "__attribute__((section(\".seg1\"))) unsigned long long a[100] = {0xdeadbeef12345678};" +
		"__attribute__((section(\".seg2\"))) unsigned long long b[512] = {0x0114051419190810};" +
		"__attribute__((section(\".seg3\"))) unsigned long long c[262144];" +
		"int _start() { return c[0] = a[0] ^ b[0]; }"
	err := ioutil.WriteFile("/tmp/3a.c", []byte(code), 0o755)
	if err != nil {
		panic(err)
	}
	cmd := exec.Command("riscv64-elf-gcc", "/tmp/3a.c", "-o", "/tmp/3a",
		"-nostdlib", "-nodefaultlibs", "-fno-builtin",
		"-march=rv64im", "-mabi=lp64",
		"-Wl,--gc-sections", "-fPIE", "-s",
		"-Ttext", "0x10000190",
		"-Wl,--section-start,.seg1=0x20000000",
		"-Wl,--section-start,.seg2=0x30000000",
		"-Wl,--section-start,.seg3=0x40000000",
	)
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
	elf, err := ioutil.ReadFile("/tmp/3a")
	if err != nil {
		panic(err)
	}
	return elf
}

func TestParseELF(t *testing.T) {
	elf := getTestELF()
	e, err := ParseELF(elf)
	if err != nil {
		t.Fatalf("error happened: %v", err)
	}
	if e.Entry != 0x10000190 {
		t.Fatalf("entry mismatch: %d", e.Entry)
	}
	if e.Segments[0].Privileges != 5 || e.Segments[0].Offset != 0 || e.Segments[0].Addr != 0x10000000 {
		t.Fatalf("segment 0 mismatch: %v", e.Segments[0])
	}
	if !reflect.DeepEqual(e.Segments[1], Segment{Privileges: 6, Offset: 0x1000, Addr: 0x20000000, FileSz: 800, MemSz: 800}) {
		t.Fatalf("segment 1 mismatch: %v", e.Segments[1])
	}
	if !reflect.DeepEqual(e.Segments[2], Segment{Privileges: 6, Offset: 0x2000, Addr: 0x30000000, FileSz: 4096, MemSz: 4096}) {
		t.Fatalf("segment 2 mismatch: %v", e.Segments[2])
	}
	if !reflect.DeepEqual(e.Segments[3], Segment{Privileges: 6, Offset: 0x3000, Addr: 0x40000000, FileSz: 1 << 21, MemSz: 1 << 21}) {
		t.Fatalf("segment 3 mismatch: %v", e.Segments[3])
	}
}

func TestTrimELF(t *testing.T) {
	elf := getTestELF()
	e, _ := ParseELF(elf)
	s1, err := TrimELF(elf, e, nil, uint64(e.Entry))
	if err != nil {
		t.Fatalf("error happened: %v", err)
	}
	e1, err := ParseELF(s1)
	if err != nil {
		t.Fatalf("error happened: %v", err)
	}
	for i := 0; i < 4; i++ {
		f := false
		f = f || e.Segments[i].Privileges != e1.Segments[i].Privileges
		f = f || e.Segments[i].Addr != e1.Segments[i].Addr
		f = f || e.Segments[i].MemSz != e1.Segments[i].MemSz
		f = f || e.Segments[i].FileSz < e1.Segments[i].FileSz
		if i != 0 {
			f = f || !bytes.Equal(
				elf[e.Segments[i].Offset:e.Segments[i].Offset+e1.Segments[i].FileSz],
				s1[e1.Segments[i].Offset:e1.Segments[i].Offset+e1.Segments[i].FileSz],
			)
		}
		if f {
			t.Fatalf("segment %d mismatch: %v %v", i, e.Segments[i], e1.Segments[i])
		}
	}
	if len(s1) > 1000 {
		t.Fatalf("not trimed")
	}
	s2, err := TrimELF(elf, e, []uint32{0x30000000}, 0x114514)
	if err != nil {
		t.Fatalf("error happened: %v", err)
	}
	e2, err := ParseELF(s2)
	if err != nil {
		t.Fatalf("error happened: %v", err)
	}
	for i := 0; i < 3; i++ {
		j := i
		if i == 2 {
			j = 3
		}
		f := false
		f = f || e.Segments[j].Privileges != e2.Segments[i].Privileges
		f = f || e.Segments[j].Addr != e2.Segments[i].Addr
		f = f || e.Segments[j].MemSz != e2.Segments[i].MemSz
		f = f || e.Segments[j].FileSz < e2.Segments[i].FileSz
		if i != 0 {
			f = f || !bytes.Equal(
				elf[e.Segments[j].Offset:e.Segments[j].Offset+e2.Segments[i].FileSz],
				s1[e2.Segments[i].Offset:e2.Segments[i].Offset+e2.Segments[i].FileSz],
			)
		}
		if f {
			t.Fatalf("segment %d mismatch: %v %v", i, e.Segments[j], e2.Segments[i])
		}
	}
	if e2.Entry != 0x114514 {
		t.Fatalf("entry not updated")
	}
	if len(s2) > 1000 {
		t.Fatalf("not trimed")
	}
}
