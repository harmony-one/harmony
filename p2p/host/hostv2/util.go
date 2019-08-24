package hostv2

import "github.com/harmony-one/harmony/pkg/utils"

func catchError(err error) {
	if err != nil {
		utils.Logger().Panic().Err(err).Msg("catchError")
	}
}
