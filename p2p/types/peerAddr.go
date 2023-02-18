package p2ptypes

import (
	"context"
	"fmt"
	"strings"
	"time"

	libp2p_peer "github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
)

// AddrList is a list of multi address
type AddrList []ma.Multiaddr

// String is a function to print a string representation of the AddrList
func (al *AddrList) String() string {
	strs := make([]string, len(*al))
	for i, addr := range *al {
		strs[i] = addr.String()
	}
	return strings.Join(strs, ",")
}

// Set is a function to set the value of AddrList based on a string
func (al *AddrList) Set(value string) error {
	if len(*al) > 0 {
		return fmt.Errorf("AddrList is already set")
	}
	for _, a := range strings.Split(value, ",") {
		addr, err := ma.NewMultiaddr(a)
		if err != nil {
			return err
		}
		*al = append(*al, addr)
	}
	return nil
}

// StringsToMultiAddrs convert a list of strings to a list of multiaddresses
func StringsToMultiAddrs(addrStrings []string) (maddrs []ma.Multiaddr, err error) {
	for _, addrString := range addrStrings {
		addr, err := ma.NewMultiaddr(addrString)
		if err != nil {
			return maddrs, err
		}
		maddrs = append(maddrs, addr)
	}
	return
}

// ResolveAndParseMultiAddrs resolve the DNS multi peer and parse to libp2p AddrInfo
func ResolveAndParseMultiAddrs(addrStrings []string) ([]libp2p_peer.AddrInfo, error) {
	var res []libp2p_peer.AddrInfo
	for _, addrStr := range addrStrings {
		ais, err := resolveMultiAddrString(addrStr)
		if err != nil {
			return nil, err
		}
		res = append(res, ais...)
	}
	return res, nil
}

func resolveMultiAddrString(addrStr string) ([]libp2p_peer.AddrInfo, error) {
	var ais []libp2p_peer.AddrInfo

	mAddr, err := ma.NewMultiaddr(addrStr)
	if err != nil {
		return nil, err
	}
	mAddrs, err := resolveMultiAddr(mAddr)
	if err != nil {
		return nil, err
	}
	for _, mAddr := range mAddrs {
		ai, err := libp2p_peer.AddrInfoFromP2pAddr(mAddr)
		if err != nil {
			return nil, err
		}
		ais = append(ais, *ai)
	}
	return ais, nil
}

func resolveMultiAddr(raw ma.Multiaddr) ([]ma.Multiaddr, error) {
	if madns.Matches(raw) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		mas, err := madns.Resolve(ctx, raw)
		if err != nil {
			return nil, err
		}
		return mas, nil
	}
	return []ma.Multiaddr{raw}, nil
}
