package common

import (
	"encoding/csv"
	"os"
	"strings"

	"github.com/harmony-one/harmony/internal/utils"
)

var defaultFix *LocalFix

func init() {
	defaultFix = &LocalFix{
		txForceSuccess: make(map[string]bool),
		txForceFailed:  make(map[string]bool),
	}
}

func InitRosettaFile(file string) {
	defaultFix.fixFile = file
	defaultFix.init()
}

func GetDefaultFix() *LocalFix {
	return defaultFix
}

type LocalFix struct {
	txForceSuccess map[string]bool
	txForceFailed  map[string]bool
	fixFile        string
}

func (f *LocalFix) init() {
	if _, err := os.Stat(f.fixFile); !os.IsNotExist(err) {
		fixCsv, err := os.Open(f.fixFile)
		if err != nil {
			return
		}
		defer fixCsv.Close()

		reader := csv.NewReader(fixCsv)
		count := 0
		for {
			read, err := reader.Read()
			if err != nil {
				break
			}

			if len(read) != 2 {
				continue
			}

			switch read[0] {
			case "txForceSuccess":
				f.txForceSuccess[strings.ToLower(read[1])] = true
				count++
				break
			case "txForceFailed":
				f.txForceFailed[strings.ToLower(read[1])] = true
				count++
				break
			}
		}

		utils.Logger().Debug().Msgf("Using rosetta fix file at `%s`, read %d line", f.fixFile, count)
	}
}

func (f *LocalFix) IsForceTxSuccess(txHash string) bool {
	return f.txForceSuccess[strings.ToLower(txHash)]
}

func (f *LocalFix) IsForceTxFailed(txHash string) bool {
	return f.txForceFailed[strings.ToLower(txHash)]
}
