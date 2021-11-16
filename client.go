package mcclient

import (
	"fmt"
)

// Varint is documented at https://wiki.vg/Protocol#VarInt_and_VarLong
// It's a littlendian byte slice constructed of 7-bit groups with the MSB indicating continuation

const maxVarintBytes = 5

type Varint struct{ value []byte }

// Constructor: New converts from int32 to Variant
func NewVarint(input int32) *Varint {
	var buf [maxVarintBytes]byte // buf is a blank buffer for the Varint
	var n int
	x := uint32(input)
	for n = 0; x&0b11111111111111111111111110000000 != 0; n++ { // loop through the buf until we can fit into the last byte
		buf[n] = 0b10000000 | byte(x&0b01111111) // Copy the least significant 7 bits and set the flag
		x >>= 7                                  // shift the input throwing away the 7 bits we just packed
	}
	buf[n] = byte(x)
	return &Varint{buf[0 : n+1]}
}

// ToVar updates an existing Varint with a new value converted from fixed
func (v Varint) FromInt(input int32) {
	var buf [maxVarintBytes]byte // buf is a blank buffer for the Varint
	var n int
	x := uint32(input)
	for n = 0; x&0b11111111111111111111111110000000 != 0; n++ { // loop through the buf until we can fit into the last byte
		buf[n] = 0b10000000 | byte(x&0b01111111) // Copy the least significant 7 bits and set the flag
		x >>= 7                                  // shift the input throwing away the 7 bits we just packed
	}
	buf[n] = byte(x)
	copy(v.value, buf[0:n+1])
}

// ToInt converts to fixed-width int32
func (v Varint) ToInt() (x int32, err error) {
	n := 0
	for shift := uint(0); shift < 64; shift += 7 {
		b := int32(v.value[n])
		n++
		x |= (b & 0b01111111) << shift // Get the lower 7 bits; shift it to correct position; OR it into x
		if (b & 0b10000000) == 0 {     // have we collected all the data?
			return x, nil
		}
	}
	return 0, fmt.Errorf("couldn't unpack varint %v", v.value)
}

type McString struct {
	length Varint
	value  []byte
}

func NewMcstring(value string) *McString {
	length := NewVarint(int32(len(value)))
	return &McString{length: *length, value: []byte(value)}
}

func (m McString) ToString() string {
	return string(m.value)
}

// Ping opens a session with the target server (url), sends a status request (serverping) and returns the JSON results
func Ping(url string) {

}
