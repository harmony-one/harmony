package hostv2

import (
	"bufio"
	"hash/fnv"
	"math/rand"

	"github.com/harmony-one/harmony/log"
	ic "github.com/libp2p/go-libp2p-crypto"
)

func catchError(err error) {
	if err != nil {
		log.Error("catchError", "err", err)
		panic(err)
	}
}

func addrToPrivKey(addr string) ic.PrivKey {
	h := fnv.New32a()
	_, err := h.Write([]byte(addr))
	catchError(err)
	r := rand.New(rand.NewSource(int64(h.Sum32()))) // Hack: forcing the random see to be the hash of addr so that we can recover priv from ip + port.
	priv, _, err := ic.GenerateKeyPairWithReader(ic.RSA, 512, r)
	return priv
}

func writeData(w *bufio.Writer, data []byte) {
	w.Write(data)
	w.Flush()
}
