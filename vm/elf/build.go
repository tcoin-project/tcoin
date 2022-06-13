package elf

import (
	"io/ioutil"
	"os/exec"
)

func debugBuildELFWithFilename(source, filename, binname string) []byte {
	err := ioutil.WriteFile(filename, []byte(source), 0o755)
	if err != nil {
		panic(err)
	}
	args := []string{
		filename, "-o", binname,
		"-nostdlib", "-nodefaultlibs", "-fno-builtin",
		"-march=rv64im", "-mabi=lp64",
		"-Wl,--gc-sections", "-s",
		"-Ttext", "0x10000190",
		"-Wl,--section-start,.private_data=0x20000000",
		"-Wl,--section-start,.shared_data=0x40000000",
		"-Wl,--section-start,.init_code=0x100FF000",
	}
	if filename[len(filename)-1] == 'c' {
		args = append(args, "-fPIE")
	}
	cmd := exec.Command("riscv64-elf-gcc", args...)
	err = cmd.Run()
	if err != nil {
		panic(err)
	}
	res, err := ioutil.ReadFile(binname)
	if err != nil {
		panic(err)
	}
	return res
}

func DebugBuildELF(source string) []byte {
	return debugBuildELFWithFilename(source, "/tmp/1a.c", "/tmp/1a")
}

func DebugBuildAsmELF(source string) []byte {
	return debugBuildELFWithFilename(source, "/tmp/2a.S", "/tmp/2a")
}
