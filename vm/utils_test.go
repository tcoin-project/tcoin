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

func TestInstGen(t *testing.T) {
	// sub x5, x8, x30
	assertEq(t, genRType(0b0110011, 5, 0b000, 8, 30, 0b0100000), uint32(1105461939), "R-Type error")
	// xor x4, x27, x9
	assertEq(t, genRType(0b0110011, 4, 0b100, 27, 9, 0b0000000), uint32(10338867), "R-Type error")
	// lbu x14, -375(x11)
	assertEq(t, genIType(0b0000011, 14, 0b100, 11, -375), uint32(3902129923), "I-Type error")
	// sltiu x23, x15, 897
	assertEq(t, genIType(0b0010011, 23, 0b011, 15, 897), uint32(941079443), "I-Type error")
	// sw x17, 3(x28)
	assertEq(t, genSType(0b0100011, 0b010, 28, 17, 3), uint32(18751907), "S-Type error")
	// bge x7, x31, _start-4000
	assertEq(t, genBType(0b1100011, 0b101, 7, 31, -4000), uint32(2280902755), "B-Type error")
	// bne x5, x0, _start+2482
	assertEq(t, genBType(0b1100011, 0b001, 5, 0, 2482), uint32(436378083), "B-Type error")
	// lui x19, 675249 (-1529147392)
	assertEq(t, genUType(0b0110111, 19, -1529147392), uint32(2765822391), "U-Type error")
	// jal x6, _start+804806
	assertEq(t, genJType(0b1101111, 6, 804806), uint32(2087469935), "J-Type error")
	// jal x17, _start-576118
	assertEq(t, genJType(0b1101111, 17, -576118), uint32(3634837743), "J-Type error")
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
