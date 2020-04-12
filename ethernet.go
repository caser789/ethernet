// Package ethernet implements marshaling and unmarshaling of IEEE 802.3
// Ethernet II frames and IEEE 802.10 VLAN tags.
package ethernet

import (
	"encoding/binary"
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
	// DestinationHardwareAddr specifies the destination hardware address for this Frame.
	// If this address is set to Broadcast, the Frame will be sent to every
	// device on a given LAN segment.
	DestinationHardwareAddr net.HardwareAddr

	// SourceHardwareAddr specifies the source hardware address for this Frame. Typically,
	// this hardware address is the address of the network interface used to send
	// this Frame.
	SourceHardwareAddr net.HardwareAddr

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
	// 6 bytes: destination hardware address
	// 6 bytes: source hardware address
	// N bytes: 4 * N VLAN tags
	// 2 bytes: EtherType
	// N bytes: payload length (may be padded)
	//
	// We let the operating system handle the checksum and the interpacket gap

	// If payload is less than the required min length, we zero-pad up to
	// the required min length
	pl := len(f.Payload)
	if pl < minPayload {
		pl = minPayload
	}

	b := make([]byte, 6+6+(4*len(f.VLAN))+2+pl)

	copy(b[0:6], f.DestinationHardwareAddr)
	copy(b[6:12], f.SourceHardwareAddr)

	// Marshal each VLAN tag into bytes, inserting a VLAN EtherType value
	// before each, so device know that one or more VLANs are present.
	n := 12
	for _, v := range f.VLAN {
		// Add VLAN EtherType and VLAN bytes
		binary.BigEndian.PutUint16(b[n:n+2], uint16(EtherTypeVLAN))

		if _, err := v.read(b[n+2 : n+4]); err != nil {
			return nil, err
		}

		n += 4
	}

	// Marshal actual EtherType after any VLANs, copy payload into
	// output bytes.
	// TODO why not copy here?
	binary.BigEndian.PutUint16(b[n:n+2], uint16(f.EtherType))
	copy(b[n+2:], f.Payload)

	return b, nil
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

	dst := make(net.HardwareAddr, 6)
	copy(dst, b[0:6])
	f.DestinationHardwareAddr = dst

	src := make(net.HardwareAddr, 6)
	copy(src, b[6:12])
	f.SourceHardwareAddr = src

	// Track offset in packet for writing data
	n := 14

	// Continue looping and parsing VLAN tags until no more VLAN EtherType
	// values are detected
	et := EtherType(binary.BigEndian.Uint16(b[n-2 : n]))
	for ; et == EtherTypeVLAN; n += 4 {
		// 2 or more bytes must remain for valid VLAN tag
		if len(b[n:]) < 2 {
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

	// Payload must be 46 bytes minimum, but the required number decreases
	// to 42 if a VLAN tag is present.
	//
	// Special case: the OS will likely automatically remove VLAN
	// tags before we get ahold of the traffic. If the packet length seems to
	// indicate that a VLAN tag was present (42 bytes payload instead of 46
	// bytes), but no VLAN tags were detected, we relax the minimum length
	// restriction and act as if a VLAN tag was detected.

	// Check how many bytes under minimum the payload is
	l := minPayload - len(b[n:])

	// Check for number of VLANs detected, but only use 1 to reduce length
	// requirement if more than 1 is present
	vl := len(f.VLAN)
	if vl > 1 {
		vl = 1
	}

	// If no VLANs detected and exactly 4 bytes below requirement, a VLAN tag
	// may have been stripped, so factor a single VLAN tag into the minimum length
	// requirement
	if vl == 0 && l == 4 {
		vl++
	}

	if len(b[n:]) < minPayload-(vl*4) {
		return io.ErrUnexpectedEOF
	}

	payload := make([]byte, len(b[n:]))
	copy(payload, b[n:])
	f.Payload = payload

	return nil
}
