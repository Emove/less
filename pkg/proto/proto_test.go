package proto

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestGenericProtocol_BodySize(t *testing.T) {
	proto := GenericProtocol
	value := uint32(math.MaxUint32)
	header := make([]byte, proto.HeaderLength())
	binary.BigEndian.PutUint32(header, value)

	if value != proto.BodySize(header) {
		t.Fatal()
	}
}
