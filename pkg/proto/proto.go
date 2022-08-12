package proto

import "encoding/binary"

type Protocol interface {
	HeaderLength() uint32
	BodySize(header []byte) uint32
	ByteOrder() binary.ByteOrder
}

var GenericProtocol Protocol

func init() {
	GenericProtocol = &genericProtocol{}
}

type genericProtocol struct{}

func (p *genericProtocol) HeaderLength() uint32 {
	return 4
}

func (p *genericProtocol) BodySize(header []byte) uint32 {
	return binary.BigEndian.Uint32(header)
}

func (p *genericProtocol) ByteOrder() binary.ByteOrder {
	return binary.BigEndian
}
