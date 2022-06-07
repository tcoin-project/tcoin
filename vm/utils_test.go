package vm

import (
	"math/rand"
	"reflect"
	"testing"
)

func assertEq(t *testing.T, a, b interface{}, msg string, args ...interface{}) {
	if !reflect.DeepEqual(a, b) {
		args = append(args, a)
		args = append(args, b)
		t.Fatalf(msg+": %v != %v", args...)
	}
}

func assertNe(t *testing.T, a, b interface{}, msg string, args ...interface{}) {
	if a == nil && b == nil {
		args = append(args, a)
		args = append(args, b)
		t.Fatalf(msg+": %v == %v", args...)
	} else if a != nil && b != nil {
		aType := reflect.TypeOf(a).Kind()
		bType := reflect.TypeOf(b).Kind()
		if aType != bType {
			args = append(args, aType)
			args = append(args, bType)
			t.Fatalf(msg+": type mismatch (%v != %v)", args...)
		} else if reflect.DeepEqual(a, b) {
			args = append(args, a)
			args = append(args, b)
			t.Fatalf(msg+": %v == %v", args...)
		}
	}
}

func TestInstGen(t *testing.T) {
	assertEq(t, genRType(0b0110011, 5, 0b000, 8, 30, 0b0100000), asmToInt("sub x5, x8, x30"), "R-Type error")
	assertEq(t, genRType(0b0110011, 4, 0b100, 27, 9, 0b0000000), asmToInt("xor x4, x27, x9"), "R-Type error")
	assertEq(t, genIType(0b0000011, 14, 0b100, 11, -375), asmToInt("lbu x14, -375(x11)"), "I-Type error")
	assertEq(t, genIType(0b0010011, 23, 0b011, 15, 897), asmToInt("sltiu x23, x15, 897"), "I-Type error")
	assertEq(t, genSType(0b0100011, 0b010, 28, 17, 3), asmToInt("sw x17, 3(x28)"), "S-Type error")
	assertEq(t, genBType(0b1100011, 0b101, 7, 31, -4000), asmToInt("bge x7, x31, _start-4000"), "B-Type error")
	assertEq(t, genBType(0b1100011, 0b001, 5, 0, 2482), asmToInt("bne x5, x0, _start+2482"), "B-Type error")
	assertEq(t, genUType(0b0110111, 19, -1529147392), asmToInt("lui x19, 675249"), "U-Type error")
	assertEq(t, genJType(0b1101111, 6, 804806), asmToInt("jal x6, _start+804806"), "J-Type error")
	assertEq(t, genJType(0b1101111, 17, -576118), asmToInt("jal x17, _start-576118"), "J-Type error")
}

func TestImmExtract(t *testing.T) {
	rnd := rand.New(rand.NewSource(114514))
	// I-Type
	for i := 0; i < 100; i++ {
		x := int32(rnd.Intn(4096) - 2048)
		y := ImmIType(genIType(0, 0, 0, 0, x))
		assertEq(t, uint32(x), y, "I-Type error")
	}
	// S-Type
	for i := 0; i < 100; i++ {
		x := int32(rnd.Intn(4096) - 2048)
		y := ImmSType(genSType(0, 0, 0, 0, x))
		assertEq(t, uint32(x), y, "S-Type error")
	}
	// B-Type
	for i := 0; i < 100; i++ {
		x := int32(rnd.Intn(4096)-2048) * 2
		y := ImmBType(genBType(0, 0, 0, 0, x))
		assertEq(t, uint32(x), y, "B-Type error")
	}
	// U-Type
	for i := 0; i < 100; i++ {
		x := int32(rnd.Intn(1048576)-524288) * 4096
		y := ImmUType(genUType(0, 0, x))
		assertEq(t, uint32(x), y, "U-Type error")
	}
	// J-Type
	for i := 0; i < 100; i++ {
		x := int32(rnd.Intn(1048576)-524288) * 2
		y := ImmJType(genJType(0, 0, x))
		assertEq(t, uint32(x), y, "J-Type error")
	}
}
