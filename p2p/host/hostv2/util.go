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

func stringsToAddrs(addrStrings []string) (maddrs []maddr.Multiaddr, err error) {
	for _, addrString := range addrStrings {
		addr, err := maddr.NewMultiaddr(addrString)
		if err != nil {
			return maddrs, err
		}
		maddrs = append(maddrs, addr)
	}
	return
}
