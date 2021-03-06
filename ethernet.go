// Package ethernet implements marshaling and unmarshaling of IEEE 802.3
// Ethernet II frames and IEEE 802.10 VLAN tags.
package ethernet

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"net"
)

//go:generate stringer -output=string.go -type=EtherType

const (
	// minPayload is the minimum payload size for an Ethernet frame, assuming
	// that no 802.1Q VLAN tags are present
	minPayload = 46
)

var (
	// Broadcast is a special hardware address which indicates a Frame should be
	// sent to every device on a given LAN segment.
	Broadcast = net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	// ErrInvalidFCS is returned when Frame.UnmarshalFCS detects an incorrect
	// Ethernet frame check sequence in a byte slice for a Frame.
	ErrInvalidFCS = errors.New("invalid frame check sequence")
)

// An EtherType is a value used to identify an upper layer protocol
// encapsulated in a Frame
type EtherType uint16

// CommonEtherType values frequently used in a Frame
const (
	EtherTypeIPv4 EtherType = 0x0800
	EtherTypeARP  EtherType = 0x0806
	EtherTypeVLAN EtherType = 0x8100
	EtherTypeIPv6 EtherType = 0x86DD
)

// A Frame is an IEEE 802.3 Ethernet II frame. A Frame contains information
// such as source and destination hardware addresses, zero or more optional 802.1Q
// VLAN tags, an EtherType, and payload data.
type Frame struct {
	// Destination specifies the destination hardware address for this Frame.
	// If this address is set to Broadcast, the Frame will be sent to every
	// device on a given LAN segment.
	Destination net.HardwareAddr

	// Source specifies the source hardware address for this Frame. Typically,
	// this hardware address is the address of the network interface used to send
	// this Frame.
	Source net.HardwareAddr

	// Vlan specifies one or more optional 802.1Q VLAN tags, which many or may
	// not be present in a Frame. If no VLAN tags are present, this length of
	// the slice will be 0.
	VLAN []*VLAN

	// EtherType is a value used to identify an upper layer protocol
	// encapsulated in this Frame.
	EtherType EtherType

	// Payload is a variable length data payload encapsulated by this Frame
	Payload []byte
}

// MarshalBinary allocates a byte slice and marshals a Frame into binary form.
//
// If one or more VLANs are set and their IDs are too large (greater than 4094),
// or one or more VLANs' priority are too large (greater than 7),
// ErrInvalidVLAN is returned
func (f *Frame) MarshalBinary() ([]byte, error) {
	b := make([]byte, f.length())
	_, err := f.read(b)
	return b, err
}

// MarshalFCS allocates a byte slice, marshals a Frame into binary form, and
// finally calculates and places a 4-byte IEEE CRC32 frame check sequence at
// the end of the slice
func (f *Frame) MarshalFCS() ([]byte, error) {
	// Frame length with 4 extra bytes for frame check sequence
	b := make([]byte, f.length()+4)
	if _, err := f.read(b); err != nil {
		return nil, err
	}

	binary.BigEndian.PutUint32(b[len(b)-4:], crc32.ChecksumIEEE(b[0:len(b)-4]))
	return b, nil
}

// read reads data from a Frame into b. read is used to marshal a Frame
// into a binary form, but does not allocate on its own
func (f *Frame) read(b []byte) (int, error) {
	copy(b[0:6], f.Destination)
	copy(b[6:12], f.Source)

	// Marshal each VLAN tag into bytes, inserting a VLAN EtherType value
	// before each, so device know that one or more VLANs are present.
	n := 12
	for _, v := range f.VLAN {
		// Add VLAN EtherType and VLAN bytes
		binary.BigEndian.PutUint16(b[n:n+2], uint16(EtherTypeVLAN))

		if _, err := v.read(b[n+2 : n+4]); err != nil {
			return 0, err
		}

		n += 4
	}

	// Marshal actual EtherType after any VLANs, copy payload into
	// output bytes.
	// TODO why not copy here?
	binary.BigEndian.PutUint16(b[n:n+2], uint16(f.EtherType))
	copy(b[n+2:], f.Payload)

	return len(b), nil
}

// UnmarshalBinary unmarshals a byte slice into a Frame
//
// If the byte slice does not contain enough data to unmarshal a valid Frame,
// io.ErrUnexpectedEOF is returned.
//
// If one or more VLANs are detected and their IDs are too large (greater than
// 4094), ErrInvalidVLAN is returned
func (f *Frame) UnmarshalBinary(b []byte) error {
	// Verify that both hardware addresses and a single EtherType are present
	if len(b) < 14 {
		return io.ErrUnexpectedEOF
	}

	// Track offset in packet for reading data
	n := 14

	// Continue looping and parsing VLAN tags until no more VLAN EtherType
	// values are detected
	et := EtherType(binary.BigEndian.Uint16(b[n-2 : n]))
	for ; et == EtherTypeVLAN; n += 4 {
		// 4 or more bytes must remain for valid VLAN tag and EtherType
		if len(b[n:]) < 4 {
			return io.ErrUnexpectedEOF
		}

		// Body of VLAN tag is 2 bytes in length;
		vlan := new(VLAN)

		if err := vlan.UnmarshalBinary(b[n : n+2]); err != nil {
			return err
		}
		f.VLAN = append(f.VLAN, vlan)

		// Parse next tag to determine if it is another VLAN, or if not,
		// break the loop
		et = EtherType(binary.BigEndian.Uint16(b[n+2 : n+4]))
	}
	f.EtherType = et

	// Allocate single byte slice to store destination and source hardware
	// addresses, and payload
	bb := make([]byte, 6+6+len(b[n:]))
	copy(bb[0:6], b[0:6])
	f.Destination = bb[0:6]
	copy(bb[6:12], b[6:12])
	f.Source = bb[6:12]

	// There used to be a minimum length restriction here, but as
	// long as two hardware addresses and an EtherType are present, it
	// doesn't really matter what is contained in the payload. We will
	// follow the "robustness principle".
	copy(bb[12:], b[n:])
	f.Payload = bb[12:]

	return nil
}

// UnmarshalFCS computes the IEEE CRC32 frame check sequence of a Frame,
// verifies it against the checksum present in the byte slice, and finally,
// unmarshals a byte slice into a Frame
func (f *Frame) UnmarshalFCS(b []byte) error {
	// Must contain enough data for FCS, to avoid panics
	if len(b) < 4 {
		return io.ErrUnexpectedEOF
	}

	want := binary.BigEndian.Uint32(b[len(b)-4:])
	got := crc32.ChecksumIEEE(b[0 : len(b)-4])
	if want != got {
		return ErrInvalidFCS
	}

	return f.UnmarshalBinary(b[0 : len(b)-4])
}

func (f *Frame) length() int {
	pl := len(f.Payload)
	if pl < minPayload {
		pl = minPayload
	}

	return 6 + 6 + (4 * len(f.VLAN)) + 2 + pl
}
