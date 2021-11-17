package mcclient

import (
	"bytes"
	"fmt"
	"testing"
)

func TestwriteVarInt() {

}

func TestVarInt(t *testing.T) {

	// Check that NewVarint computes examples from https://wiki.vg/Protocol#VarInt_and_VarLong correctly

	type testTableEntry struct {
		fixed    int32
		variable []byte
	}

	testTable := [...]testTableEntry{
		{fixed: 0, variable: []byte{0}},
		{fixed: 128, variable: []byte{0x80, 0x01}},
		{fixed: 2097151, variable: []byte{0xff, 0xff, 0x7f}},
		{fixed: -1, variable: []byte{0xff, 0xff, 0xff, 0xff, 0x0f}},
		{fixed: -2147483648, variable: []byte{0x80, 0x80, 0x80, 0x80, 0x08}},
	}
	for _, row := range testTable {
		// Test constructor of Varint given fixed value
		v := NewVarint(row.fixed)
		if !bytes.Equal(v.value, row.variable) {
			t.Errorf("expected %x for NewVarint but got %x for %d", row.variable, v.value, row.fixed)
		}
		if v.length != len(row.variable) {
			t.Errorf("Varint length stored as %d, but should be %d", v.length, len(row.variable))
		}

		// Test constructor of Varint given byte stream
		v, err := ReadVarint(bytes.NewReader(row.variable))
		if err != nil {
			t.Errorf("error reading Varint bytes: %v", err)
		}
		if !bytes.Equal(v.value, row.variable) {
			t.Errorf("expected %x for NewVarint but got %x for %d", row.variable, v.value, row.fixed)
		}
		if (v.length) != len(row.variable) {
			t.Errorf("Varint length stored as %d, but should be %d", v.length, len(row.variable))
		}

		// Test conversion of Varint to fixed int value
		x, _ := v.ToInt()
		if x != row.fixed {
			t.Errorf("expected %d, but got %v converting %x to fixed", row.fixed, x, v.value)
		}

		// Test converstion of fixed to Varint (updating value, not allocating new)
		v.FromInt(row.fixed)
		if !bytes.Equal(v.value, row.variable) {
			t.Errorf("expected %x for Varint.FromInt but got %x for %d", v.value, row.variable, row.fixed)
		}
	}

	// Test string conversion to/from Minecraft format

	string1 := "This is a test string"
	mc1 := NewMcstring(string1)
	varlen, _ := mc1.Length.ToInt()
	if varlen != int32(len(string1)) {
		t.Errorf(`expected length %d, got %d for test string "%s"`, len(string1), varlen, string1)
	}

	string2 := mc1.ToString()
	if string1 != string2 {
		t.Errorf(`mangled strings: "%s" should match "%s"`, string1, string2)
	}

	// Test Ping

	//	fmt.Println("\nTesting ping")
	url := "13.40.28.10:25565"
	//	url := "localhost:25565"
	res, err := Ping(url)
	if err != nil {
		t.Errorf("pinging %s: %v", url, err)
	}
	fmt.Printf("\n%s", res)

}
