package hostv2

import (
	"bufio"

	"github.com/harmony-one/harmony/log"
	maddr "github.com/multiformats/go-multiaddr"
)

func catchError(err error) {
	if err != nil {
		log.Error("catchError", "err", err)
		panic(err)
	}
}

func writeData(w *bufio.Writer, data []byte) {
	w.Write(data)
	w.Flush()
}

func stringToAddr(addrString string) (addr maddr.Multiaddr, err error) {
	addr, err = maddr.NewMultiaddr(addrString)
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func stringsToAddrs(addrStrings []string) (maddrs []maddr.Multiaddr, err error) {
	for _, addrString := range addrStrings {
		addr, err := stringToAddr(addrString)
		if err != nil {
			return maddrs, err
		}
		maddrs = append(maddrs, addr)
	}
	return
}
