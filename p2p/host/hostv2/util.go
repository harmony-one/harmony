package hostv2

import (
	"bufio"

	"github.com/harmony-one/harmony/log"
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
