package p2pv2

import (
	"bufio"
	"math/rand"
	"strconv"

	"github.com/harmony-one/harmony/log"
	ic "github.com/libp2p/go-libp2p-crypto"
)

func catchError(err error) {
	if err != nil {
		log.Error("catchError", "err", err)
		panic(err)
	}
}

func portToPrivKey(port string) ic.PrivKey {
	portNum, err := strconv.ParseInt(port, 10, 64)
	catchError(err)
	r := rand.New(rand.NewSource(portNum))
	priv, _, err := ic.GenerateKeyPairWithReader(ic.RSA, 512, r)
	return priv
}

func writeData(w *bufio.Writer, data []byte) {
	for {
		w.Write(data)
		w.Flush()
	}
}
