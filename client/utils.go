package client

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/simple-rules/harmony-benchmark/crypto/pki"
)

var AddressToIntPriKeyMap map[[20]byte]int // For convenience, we use int as the secret seed for generating private key

func LookUpIntPriKey(address [20]byte) (int, bool) {
	if AddressToIntPriKeyMap == nil {
		AddressToIntPriKeyMap = make(map[[20]byte]int)
		for i := 1; i <= 10000; i++ {
			AddressToIntPriKeyMap[pki.GetAddressFromInt(i)] = i
		}
	}
	value, ok := AddressToIntPriKeyMap[address]
	return value, ok
}

func DownloadUrlAsString(url string) (string, error) {
	response, err := http.Get(url)
	buf := bytes.NewBufferString("")
	if err != nil {
		log.Fatal(err)
	} else {
		defer response.Body.Close()
		_, err := io.Copy(buf, response.Body)
		if err != nil {
			log.Fatal(err)
		}
	}
	return buf.String(), err
}
