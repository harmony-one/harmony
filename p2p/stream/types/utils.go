package sttypes

// TODO: test this file

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/hashicorp/go-version"
	libp2p_proto "github.com/libp2p/go-libp2p-core/protocol"
	"github.com/pkg/errors"
)

const (
	// ProtoIDCommonPrefix is the common prefix for stream protocol
	ProtoIDCommonPrefix = "harmony"

	// ProtoIDFormat is the format of stream protocol ID
	ProtoIDFormat = "%s/%s/%s/%d/%s"

	// protoIDNumElem is the number of elements of the ProtoID. See comments in ProtoID
	protoIDNumElem = 5
)

// ProtoID is the protocol id for streaming, an alias of libp2p stream protocol ID。
// The stream protocol ID is composed of following components:
// 1. Service - Currently, only sync service is supported.
// 2. NetworkType - mainnet, testnet, stn, e.t.c.
// 3. ShardID - shard ID of the current protocol.
// 4. Version - Stream protocol version for backward compatibility.
type ProtoID libp2p_proto.ID

// ProtoSpec is the un-serialized stream proto id specification
// TODO: move this to service wise module since different protocol might have different
//   protoID information
type ProtoSpec struct {
	Service     string
	NetworkType nodeconfig.NetworkType
	ShardID     nodeconfig.ShardID
	Version     *version.Version
}

// ToProtoID convert a ProtoSpec to ProtoID.
func (spec ProtoSpec) ToProtoID() ProtoID {
	s := fmt.Sprintf(ProtoIDFormat, ProtoIDCommonPrefix, spec.Service,
		spec.NetworkType, spec.ShardID, spec.Version.String())
	return ProtoID(s)
}

// ProtoIDToProtoSpec converts a ProtoID to ProtoSpec
func ProtoIDToProtoSpec(id ProtoID) (ProtoSpec, error) {
	comps := strings.Split(string(id), "/")
	if len(comps) != protoIDNumElem {
		return ProtoSpec{}, errors.New("unexpected protocol size")
	}
	var (
		prefix      = comps[0]
		service     = comps[1]
		networkType = comps[2]
		shardIDStr  = comps[3]
		versionStr  = comps[4]
	)
	shardID, err := strconv.Atoi(shardIDStr)
	if err != nil {
		return ProtoSpec{}, errors.Wrap(err, "invalid shard ID")
	}
	if prefix != ProtoIDCommonPrefix {
		return ProtoSpec{}, errors.New("unexpected prefix")
	}
	version, err := version.NewVersion(versionStr)
	if err != nil {
		return ProtoSpec{}, errors.Wrap(err, "unexpected version string")
	}
	return ProtoSpec{
		Service:     service,
		NetworkType: nodeconfig.NetworkType(networkType),
		ShardID:     nodeconfig.ShardID(uint32(shardID)),
		Version:     version,
	}, nil
}

// GenReqID generates a random ReqID
func GenReqID() uint64 {
	var rnd [8]byte
	rand.Read(rnd[:])
	return binary.BigEndian.Uint64(rnd[:])
}
