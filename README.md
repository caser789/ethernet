ethernet [![Build Status](https://travis-ci.org/caser789/ethernet.svg?branch=master)](https://travis-ci.org/caser789/ethernet)
[![GoDoc](https://godoc.org/github.com/caser789/ethernet?status.svg)](https://godoc.org/github.com/caser789/ethernet)
[![Go Report Card](https://goreportcard.com/badge/github.com/caser789/ethernet)](https://goreportcard.com/report/github.com/caser789/ethernet)
[![Coverage Status](https://coveralls.io/repos/caser789/ethernet/badge.svg?branch=master)](https://coveralls.io/r/caser789/ethernet?branch=master)
=====

![uml class diagram](./ethernet.png)

```
@startuml

title ethernet

class Frame {
    +Destination net.HardwareAddr
    +Source net.HardwareAddr
    +VLAN   []*VLAN
    +EtherType EtherType
    +Payload []byte
    +MarshalBinary() []byte
    +UnmarshalBinary([]byte)
    +MarshalFCS() []byte
    +UnmarshalFCS([]byte)
    -read([]byte)
    -length()
}

enum EtherType {
    IPv4
    ARP
    IPv6
    VLAN
}

enum Priority {
	Background
	BestEffort
	ExcellentEffort
    CriticalApplications
	Video
	Voice
	InternetworkControl
	NetworkControl
}

interface net.HardwareAddr {}

class VLAN {
    +Priority
    +DropEligible
    +ID
    +MarshalBinary() []byte
    -read([]byte)
    +UnmarshalBinary([]byte)
}

Frame *-- VLAN

@enduml
```
