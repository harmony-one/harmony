package reward

import (
	"sort"
	"time"

	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/numeric"
)

type pair struct {
	ts    int64
	share numeric.Dec
}

var (

	// schedule is the Token Release Schedule of Harmony
	releasePlan = map[int64]numeric.Dec{
		//2019
		mustParse("2019-May-31"): numeric.MustNewDecFromStr("0.2429"),
		mustParse("2019-Jun-30"): numeric.MustNewDecFromStr("0.2488"),
		mustParse("2019-Jul-31"): numeric.MustNewDecFromStr("0.2547"),
		mustParse("2019-Aug-31"): numeric.MustNewDecFromStr("0.2607"),
		mustParse("2019-Sep-30"): numeric.MustNewDecFromStr("0.2666"),
		mustParse("2019-Oct-31"): numeric.MustNewDecFromStr("0.2726"),
		mustParse("2019-Nov-30"): numeric.MustNewDecFromStr("0.3785"),
		mustParse("2019-Dec-31"): numeric.MustNewDecFromStr("0.3844"),
		// 2020
		mustParse("2020-Jan-31"): numeric.MustNewDecFromStr("0.3951"),
		mustParse("2020-Feb-29"): numeric.MustNewDecFromStr("0.4047"),
		mustParse("2020-Mar-31"): numeric.MustNewDecFromStr("0.4143"),
		mustParse("2020-Apr-30"): numeric.MustNewDecFromStr("0.4239"),
		mustParse("2020-May-31"): numeric.MustNewDecFromStr("0.5335"),
		mustParse("2020-Jun-30"): numeric.MustNewDecFromStr("0.5431"),
		mustParse("2020-Jul-31"): numeric.MustNewDecFromStr("0.5527"),
		mustParse("2020-Aug-31"): numeric.MustNewDecFromStr("0.5623"),
		mustParse("2020-Sep-30"): numeric.MustNewDecFromStr("0.5719"),
		mustParse("2020-Oct-31"): numeric.MustNewDecFromStr("0.5815"),
		mustParse("2020-Nov-30"): numeric.MustNewDecFromStr("0.6882"),
		mustParse("2020-Dec-31"): numeric.MustNewDecFromStr("0.6948"),
		// 2021
		mustParse("2021-Jan-31"): numeric.MustNewDecFromStr("0.7015"),
		mustParse("2021-Feb-28"): numeric.MustNewDecFromStr("0.7082"),
		mustParse("2021-Mar-31"): numeric.MustNewDecFromStr("0.7148"),
		mustParse("2021-Apr-30"): numeric.MustNewDecFromStr("0.7215"),
		mustParse("2021-May-31"): numeric.MustNewDecFromStr("0.7721"),
		mustParse("2021-Jun-30"): numeric.MustNewDecFromStr("0.7788"),
		mustParse("2021-Jul-31"): numeric.MustNewDecFromStr("0.7855"),
		mustParse("2021-Aug-31"): numeric.MustNewDecFromStr("0.7922"),
		mustParse("2021-Sep-30"): numeric.MustNewDecFromStr("0.7988"),
		mustParse("2021-Oct-31"): numeric.MustNewDecFromStr("0.8055"),
		mustParse("2021-Nov-30"): numeric.MustNewDecFromStr("0.8561"),
		mustParse("2021-Dec-31"): numeric.MustNewDecFromStr("0.8628"),
		// 2022
		mustParse("2022-Jan-31"): numeric.MustNewDecFromStr("0.8695"),
		mustParse("2022-Feb-28"): numeric.MustNewDecFromStr("0.8761"),
		mustParse("2022-Mar-31"): numeric.MustNewDecFromStr("0.8828"),
		mustParse("2022-Apr-30"): numeric.MustNewDecFromStr("0.8895"),
		mustParse("2022-May-31"): numeric.MustNewDecFromStr("0.8962"),
		mustParse("2022-Jun-30"): numeric.MustNewDecFromStr("0.9028"),
		mustParse("2022-Jul-31"): numeric.MustNewDecFromStr("0.9095"),
		mustParse("2022-Aug-31"): numeric.MustNewDecFromStr("0.9162"),
		mustParse("2022-Sep-30"): numeric.MustNewDecFromStr("0.9228"),
		mustParse("2022-Oct-31"): numeric.MustNewDecFromStr("0.9295"),
		mustParse("2022-Nov-30"): numeric.MustNewDecFromStr("0.9362"),
		mustParse("2022-Dec-31"): numeric.MustNewDecFromStr("0.9429"),
		// 2023
		mustParse("2023-Jan-31"): numeric.MustNewDecFromStr("0.9448"),
		mustParse("2023-Feb-28"): numeric.MustNewDecFromStr("0.9468"),
		mustParse("2023-Mar-31"): numeric.MustNewDecFromStr("0.9488"),
		mustParse("2023-Apr-30"): numeric.MustNewDecFromStr("0.9507"),
		mustParse("2023-May-31"): numeric.MustNewDecFromStr("0.9527"),
		mustParse("2023-Jun-30"): numeric.MustNewDecFromStr("0.9547"),
		mustParse("2023-Jul-31"): numeric.MustNewDecFromStr("0.9567"),
		mustParse("2023-Aug-31"): numeric.MustNewDecFromStr("0.9586"),
		mustParse("2023-Sep-30"): numeric.MustNewDecFromStr("0.9606"),
		mustParse("2023-Oct-31"): numeric.MustNewDecFromStr("0.9626"),
		mustParse("2023-Nov-30"): numeric.MustNewDecFromStr("0.9645"),
		mustParse("2023-Dec-31"): numeric.MustNewDecFromStr("0.9665"),
		// 2024
		mustParse("2024-Jan-31"): numeric.MustNewDecFromStr("0.9685"),
		mustParse("2024-Feb-29"): numeric.MustNewDecFromStr("0.9704"),
		mustParse("2024-Mar-31"): numeric.MustNewDecFromStr("0.9724"),
		mustParse("2024-Apr-30"): numeric.MustNewDecFromStr("0.9744"),
		mustParse("2024-May-31"): numeric.MustNewDecFromStr("0.9764"),
		mustParse("2024-Jun-30"): numeric.MustNewDecFromStr("0.9783"),
		mustParse("2024-Jul-31"): numeric.MustNewDecFromStr("0.9803"),
		mustParse("2024-Aug-31"): numeric.MustNewDecFromStr("0.9823"),
		mustParse("2024-Sep-30"): numeric.MustNewDecFromStr("0.9842"),
		mustParse("2024-Oct-31"): numeric.MustNewDecFromStr("0.9862"),
		mustParse("2024-Nov-30"): numeric.MustNewDecFromStr("0.9882"),
		mustParse("2024-Dec-31"): numeric.MustNewDecFromStr("0.9901"),
		// 2025
		mustParse("2025-Jan-31"): numeric.MustNewDecFromStr("0.9921"),
		mustParse("2025-Feb-28"): numeric.MustNewDecFromStr("0.9941"),
		mustParse("2025-Mar-31"): numeric.MustNewDecFromStr("0.9961"),
		mustParse("2025-Apr-30"): numeric.MustNewDecFromStr("0.9980"),
		mustParse("2025-May-31"): numeric.MustNewDecFromStr("1.0000"),
	}
	sorted = func() []pair {
		s := []pair{}
		for k, v := range releasePlan {
			s = append(s, pair{k, v})
		}
		sort.SliceStable(
			s,
			func(i, j int) bool { return s[i].ts < s[j].ts },
		)
		return s
	}()
)

func mustParse(ts string) int64 {
	const shortForm = "2006-Jan-02"
	t, err := time.Parse(shortForm, ts)
	if err != nil {
		panic("could not parse timestamp")
	}
	return t.Unix()
}

// PercentageForTimeStamp ..
func PercentageForTimeStamp(ts int64) numeric.Dec {
	bucket := pair{}
	i, j := 0, 1

	for range sorted {
		if i == (len(sorted) - 1) {
			if ts < sorted[0].ts {
				bucket = sorted[0]
			} else {
				bucket = sorted[i]
			}
			break
		}
		if ts >= sorted[i].ts && ts < sorted[j].ts {
			bucket = sorted[i]
			break
		}
		i++
		j++
	}

	utils.Logger().Info().
		Str("percent of total-supply used", bucket.share.Mul(numeric.NewDec(100)).String()).
		Str("for-time", time.Unix(ts, 0).String()).
		Msg("Picked Percentage for timestamp")

	return bucket.share
}
