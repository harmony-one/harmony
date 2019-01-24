package hostv2

import (
	"github.com/ethereum/go-ethereum/log"
)

func catchError(err error) {
	if err != nil {
		log.Error("catchError", "err", err)
		panic(err)
	}
}
