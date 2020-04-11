// Code generated by "stringer --output=string.go -type=EtherType"; DO NOT EDIT.

package ethernet

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[EtherTypeIPv4-2048]
	_ = x[EtherTypeARP-2054]
	_ = x[EtherTypeVLAN-33024]
	_ = x[EtherTypeIPv6-34525]
}

const (
	_EtherType_name_0 = "EtherTypeIPv4"
	_EtherType_name_1 = "EtherTypeARP"
	_EtherType_name_2 = "EtherTypeVLAN"
	_EtherType_name_3 = "EtherTypeIPv6"
)

func (i EtherType) String() string {
	switch {
	case i == 2048:
		return _EtherType_name_0
	case i == 2054:
		return _EtherType_name_1
	case i == 33024:
		return _EtherType_name_2
	case i == 34525:
		return _EtherType_name_3
	default:
		return "EtherType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}