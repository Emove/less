package proto

import "encoding/binary"

type Protocol interface {
	HeaderLength() uint32
	BodySize(header []byte) uint32
}

type GenericProtocol struct{}

func (proto *GenericProtocol) HeaderLength() uint32 {
	return 4
}

func (proto *GenericProtocol) BodySize(header []byte) uint32 {
	return binary.BigEndian.Uint32(header)
}
