package mcclient

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"time"
)

// Varint is documented at https://wiki.vg/Protocol#VarInt_and_VarLong
// It's a littlendian byte slice constructed of 7-bit groups with the MSB indicating continuation

const maxVarintBytes = 5
const timeOutSeconds = 10

type Varint struct {
	value  []byte
	length int
}

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
	return &Varint{buf[0 : n+1], n + 1}
}

// ReadVarint consumes a io.Reader, parsing input until a valid Varint is completed
func ReadVarint(input io.Reader) (*Varint, error) {
	r := bufio.NewReader(input) // Wrap the Reader to ensure it has the ByteRead() method
	var n int
	var buf [maxVarintBytes]byte
	for n = 0; n < maxVarintBytes; n++ {
		var err error
		buf[n], err = r.ReadByte()
		if err != nil { // This will include EOF, which shouldn't happen in the middle of a Varint
			return nil, fmt.Errorf("error reading varint: %v", err)
		}
		if buf[n]&0b10000000 == 0 { // Loop until the final value which has MSB unset
			break
		}
	}
	n++
	return &Varint{buf[0:n], n}, nil
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
	Length Varint
	Value  []byte
}

func NewMcstring(value string) *McString {
	length := NewVarint(int32(len(value)))
	return &McString{Length: *length, Value: []byte(value)}
}

func (m McString) ToString() string {
	return string(m.Value)
}
func (m McString) Tobytes() []byte {
	buf := m.Length.value
	buf = append(buf, m.Value...)
	return buf
}

type Packet struct { // Minecraft protcol packet, without compression
	Length Varint
	Id     Varint
	Data   []byte
}

func NewPacket(id byte, payload []byte) *Packet { // Build a packet from a byte slice payload
	idVarint := NewVarint(int32(id))
	pkt := Packet{
		Length: *NewVarint(int32(len(payload) + len(idVarint.value))),
		Id:     *idVarint,
		Data:   payload,
	}
	return &pkt
}

func (p Packet) ToBytes() []byte {
	var buf bytes.Buffer
	buf.Write(p.Length.value)
	buf.Write(p.Id.value)
	buf.Write(p.Data)
	return buf.Bytes()
}

// Ping opens a session with the target server (url), sends a status request (serverping) and returns the JSON results
func Ping(url string) (res string, err error) {

	conn, err := net.Dial("tcp", url)
	if err != nil {
		return "", fmt.Errorf("unable to connect to server %s: %v", url, err)
	}

	// Construct the handshake packet, per https://wiki.vg/Server_List_Ping

	var payload bytes.Buffer
	payload.Write(NewVarint(756).value)
	payload.Write(NewMcstring("localhost").Tobytes())
	payload.Write([]byte{0x63, 0xDD}) // server port, uint16(25565)
	payload.Write(NewVarint(1).value)
	handshakePacket := NewPacket(0, payload.Bytes())

	// and send it to the server, followed by a simple "Request" packet

	_, err = conn.Write(handshakePacket.ToBytes())
	if err != nil {
		return "", fmt.Errorf("unable to send handshake: %v", err)
	}
	_, err = conn.Write([]byte{1, 0}) // "Request" packet is trivial to construct directly
	if err != nil {
		return "", fmt.Errorf("unable to send Request: %v", err)
	}
	/*
	   // Notchian servers are said to require this ping packet before responding. But ours doesn't.
	   		pingPacket := NewPacket(1, []byte{8, 8, 8, 8, 8, 8, 8, 8})
	   		n, err = conn.Write(pingPacket.ToBytes())
	   		if err != nil {
	   			return "", fmt.Errorf("unable to send ping: %v", err)
	   		}
	   		fmt.Printf("\n%d bytes written of %d ping", n, len(pingPacket.ToBytes())) */

	// Get the response. Start by reading the packet-length, which should be a Varint followed by a 0 (ID)
	// Todo: generalise this to read any MC Protocol packet and extract function

	r := bufio.NewReader(conn)
	pktlenVarint, err := ReadVarint(r)
	if err != nil {
		return "", err
	}
	length, err := pktlenVarint.ToInt()
	if err != nil {
		return "", err
	}

	if r.Buffered() > int(length) {
		return "", fmt.Errorf("too much data: expecting packet length  %d, got %d", length, r.Buffered())
	}

	p, err := ReadVarint(r)
	if err != nil {
		return "", fmt.Errorf("couldn't read Varint packet ID %v: %v", p, err)
	}
	packetId, err := p.ToInt()
	if err != nil {
		return "", fmt.Errorf("couldn't unpack Varint packet ID %v: %v", p, err)
	}
	if packetId != 0 {
		return "", fmt.Errorf("expected packet ID 0, but got %d", packetId)
	}
	s, err := ReadVarint(r)
	if err != nil {
		return "", fmt.Errorf("couldn't read Varint string length %v: %v", s, err)
	}
	length -= int32((p.length) + s.length)
	strlength, err := s.ToInt()
	if (err != nil) || (length != strlength) {
		return "", fmt.Errorf("string length claims to be %d, but is %d", strlength, length)
	}

	// wait for all the expected data to arrive
	timeOut := time.Now().Add(time.Duration(timeOutSeconds * time.Second))
	for r.Buffered() < int(length) && time.Now().Before(timeOut) {
		time.Sleep(10 * time.Millisecond)
	}
	if !time.Now().Before(timeOut) {
		return "", fmt.Errorf("timed out waiting for response to complete")
	}
	// drain the buffer
	response := make([]byte, length)
	io.ReadFull(r, response)
	if len(response) != int(length) {
		panic(fmt.Errorf("response packet length doesn't match calculated length"))
	}

	err = conn.Close()
	if err != nil {
		return "", fmt.Errorf("closing connection: %v", err)
	}
	res = string(response)

	return res, nil
}
