// Package macaroonpb defines the serialization details of macaroon ids
// used in the macaroon-bakery.
package macaroonpb

import (
	"github.com/golang/protobuf/proto"
)

//go:generate  protoc --go_out . id.proto

// MarshalBinary implements encoding.BinaryMarshal.
func (id *MacaroonId) MarshalBinary() ([]byte, error) {
	return proto.Marshal(id)
}

// UnmarshalBinary implements encoding.UnmarshalBinary.
func (id *MacaroonId) UnmarshalBinary(data []byte) error {
	return proto.Unmarshal(data, id)
}
