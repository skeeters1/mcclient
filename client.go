package mcclient

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
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

// Write
//func (v Varint) Write(n int, err error) {}

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
	/*
		version := NewVarint(756)
		serverAddress := NewMcstring("localhost")
		serverPort := []byte{0xDD, 0x63} // bigendian encoding of uint16(25565)
		nextState := NewVarint(1)

		fmt.Printf("\nversion: %v. server address: %v", version.value, serverAddress.Tobytes())

		var buf bytes.Buffer
		buf.Write(version.value)
		buf.Write(serverAddress.Tobytes())
		buf.Write(serverPort)
		buf.Write(nextState.value)
		fmt.Printf("%v", buf)

		handshakePacket := NewPacket(0, buf.Bytes())
		handshakeBytes := handshakePacket.ToBytes()
		fmt.Printf("\nPacket:\t%v", handshakePacket)
		fmt.Printf("\nAs bytes:\t%v", handshakeBytes)

		n, err := conn.Write(handshakeBytes)
		if err != nil {
			return "", fmt.Errorf("unable to send handshake: %v", err)
		}
		fmt.Printf("\n%d bytes written of %d handshake", n, len(handshakeBytes))

		n, err = conn.Write([]byte{1, 0}) // Send "Request" packet
		if err != nil {
			return "", fmt.Errorf("unable to send Request: %v", err)
		}
		fmt.Printf("\n%d bytes written of %d request", n, len([]byte{1, 0}))

		pingPacket := NewPacket(1, []byte{8, 8, 8, 8, 8, 8, 8, 8})
		n, err = conn.Write(pingPacket.ToBytes())
		if err != nil {
			return "", fmt.Errorf("unable to send ping: %v", err)
		}
		fmt.Printf("\n%d bytes written of %d ping", n, len(pingPacket.ToBytes()))
	*/
	msg := []byte{0xfe, 0x01, 0xfa}
	n, err := conn.Write(msg)
	if err != nil {
		return "", fmt.Errorf("sending legacy ping: %v", err)
	}
	fmt.Printf("%d bytes legacy ping sent", n)
	// Get the response

	var msgData []byte
	connbuf := bufio.NewReader(conn)
	// Read the first byte and set the underlying buffer
	b, _ := connbuf.ReadByte()
	if connbuf.Buffered() > 0 {
		msgData = append(msgData, b)
		for connbuf.Buffered() > 0 {
			// read byte by byte until the buffered data is not empty
			b, err := connbuf.ReadByte()
			if err == nil {
				msgData = append(msgData, b)
			} else {
				log.Println("-------> unreadable character...", b)
			}
		}
	}
	fmt.Println(msgData)
	/*
		var response []byte
		for {
			n, err = conn.Read(response)
			if err != nil {
				return "", fmt.Errorf("reading response: %v", err)
			}
			if n > 0 {
				fmt.Printf("%v", response)
				break
			}
		} */
	err = conn.Close()
	if err != nil {
		return "", fmt.Errorf("closing connection: %v", err)
	}
	res = string(msgData)
	fmt.Printf("\n%d bytes received.\n%s", n, res)

	return res, nil
}
