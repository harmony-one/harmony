package hostv2

import "github.com/harmony-one/harmony/internal/utils"

func catchError(err error) {
	if err != nil {
		utils.GetLogger().Error("catchError", "err", err)
		panic(err)
	}
}
