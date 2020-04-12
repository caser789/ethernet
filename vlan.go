package ethernet

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	// VLANNone is a special VLAN ID which indicates that no VLAN is being
	// used in a Frame. In this case, the VLAN's other fields may be used
	// to indicate a Frame's priority
	VLANNone = 0x000

	// VLANMax is a reserved VLAN ID which may indicate a wildcard in some
	// management system, but may not configured or transmitted in a
	// VLAN tag.
	VLANMax = 0xfff
)

var (
	// ErrInvalidVLAN is returned when a VLAN ID of greater than 4094 (0xffe)
	// is detected.
	ErrInvalidVLAN = errors.New("invalid VLAN ID")
)

// A VLAN is an IEEE 802.1Q Virtual LAN (VLAN) tag. A VLAN contains
// information regarding traffic priority and a VLAN identifier for
// a given Frame.
type VLAN struct {
	// Priority specifies an IEEE 802.1p priority level.
	Priority uint8

	// DropEligible indicates if a Frame is eligible to be dropped in the
	// presence of network congestion
	DropEligible bool

	// ID specifies the VLAN ID for a Frame. ID can be any value from 0 to
	// 4094 (0x000 to 0xffe), allowing up to 4094 VLANs.
	//
	// If ID is 0 (0x000, VLANNone), no VLAN is specified, and the other fields
	// simply indicate a Frame's priority
	ID uint16
}

// MarshalBinary allocates a byte slice and marshals a VLAN into binary form.
//
// If a VLAN ID is too large (greater than 4094), ErrInvalidVLAN is returned.
func (v *VLAN) MarshalBinary() ([]byte, error) {
	// Check for VLAN ID in valid range
	if v.ID >= VLANMax {
		return nil, ErrInvalidVLAN
	}

	// 3 bits: priority
	ub := uint16(v.Priority) << 13

	// 1 bit: drop eligible
	var drop uint16
	if v.DropEligible {
		drop = 1
	}
	ub |= drop << 12

	// 12 bits: VLAN ID
	ub |= v.ID

	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, ub)

	return b, nil
}

// UnmarshalBinary unmarshals a byte slice into a Frame
//
// If the byte slice does not contain exactly 2 bytes of data,
// io.ErrUnexpectedEOF is returned
//
// If a VLAN ID is too large (greater than 4094), ErrInvalidVLAN is returned.
func (v *VLAN) UnmarshalBinary(b []byte) error {
	// VLAN tag is always 2 bytes
	if len(b) != 2 {
		return io.ErrUnexpectedEOF
	}

	// 3 bits: priority
	// 1 bits: drop eligible
	// 12 bits: VLAN ID
	ub := binary.BigEndian.Uint16(b[0:2])
	v.Priority = uint8(ub >> 13)
	v.DropEligible = ub&0x1000 != 0
	v.ID = ub & 0x0fff

	// Check for VLAN ID in valid range
	if v.ID >= VLANMax {
		return ErrInvalidVLAN
	}

	return nil
}
