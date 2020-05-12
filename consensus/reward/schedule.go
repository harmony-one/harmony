package reward

import (
	"sort"
	"time"

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
		mustParse("2019-May-31"): numeric.MustNewDecFromStr("0.242864761904762"),
		mustParse("2019-Jun-30"): numeric.MustNewDecFromStr("0.248805938375350"),
		mustParse("2019-Jul-31"): numeric.MustNewDecFromStr("0.254747114845938"),
		mustParse("2019-Aug-31"): numeric.MustNewDecFromStr("0.260688291316526"),
		mustParse("2019-Sep-30"): numeric.MustNewDecFromStr("0.266629467787115"),
		mustParse("2019-Oct-31"): numeric.MustNewDecFromStr("0.272570644257703"),
		mustParse("2019-Nov-30"): numeric.MustNewDecFromStr("0.378471820728291"),
		mustParse("2019-Dec-31"): numeric.MustNewDecFromStr("0.384412997198879"),
		// 2020
		mustParse("2020-Jan-31"): numeric.MustNewDecFromStr("0.395055284780578"),
		mustParse("2020-Feb-29"): numeric.MustNewDecFromStr("0.404667810457516"),
		mustParse("2020-Mar-31"): numeric.MustNewDecFromStr("0.414280336134453"),
		mustParse("2020-Apr-30"): numeric.MustNewDecFromStr("0.423892861811391"),
		mustParse("2020-May-31"): numeric.MustNewDecFromStr("0.533465387488328"),
		mustParse("2020-Jun-30"): numeric.MustNewDecFromStr("0.543077913165265"),
		mustParse("2020-Jul-31"): numeric.MustNewDecFromStr("0.552690438842203"),
		mustParse("2020-Aug-31"): numeric.MustNewDecFromStr("0.562302964519140"),
		mustParse("2020-Sep-30"): numeric.MustNewDecFromStr("0.571915490196078"),
		mustParse("2020-Oct-31"): numeric.MustNewDecFromStr("0.581528015873015"),
		mustParse("2020-Nov-30"): numeric.MustNewDecFromStr("0.688159365079364"),
		mustParse("2020-Dec-31"): numeric.MustNewDecFromStr("0.694830714285714"),
		// 2021
		mustParse("2021-Jan-31"): numeric.MustNewDecFromStr("0.701502063492063"),
		mustParse("2021-Feb-28"): numeric.MustNewDecFromStr("0.708173412698412"),
		mustParse("2021-Mar-31"): numeric.MustNewDecFromStr("0.714844761904761"),
		mustParse("2021-Apr-30"): numeric.MustNewDecFromStr("0.721516111111110"),
		mustParse("2021-May-31"): numeric.MustNewDecFromStr("0.772147460317459"),
		mustParse("2021-Jun-30"): numeric.MustNewDecFromStr("0.778818809523809"),
		mustParse("2021-Jul-31"): numeric.MustNewDecFromStr("0.785490158730158"),
		mustParse("2021-Aug-31"): numeric.MustNewDecFromStr("0.792161507936507"),
		mustParse("2021-Sep-30"): numeric.MustNewDecFromStr("0.798832857142856"),
		mustParse("2021-Oct-31"): numeric.MustNewDecFromStr("0.805504206349205"),
		mustParse("2021-Nov-30"): numeric.MustNewDecFromStr("0.856135555555555"),
		mustParse("2021-Dec-31"): numeric.MustNewDecFromStr("0.862806904761904"),
		// 2022
		mustParse("2022-Jan-31"): numeric.MustNewDecFromStr("0.869478253968253"),
		mustParse("2022-Feb-28"): numeric.MustNewDecFromStr("0.876149603174602"),
		mustParse("2022-Mar-31"): numeric.MustNewDecFromStr("0.882820952380951"),
		mustParse("2022-Apr-30"): numeric.MustNewDecFromStr("0.889492301587301"),
		mustParse("2022-May-31"): numeric.MustNewDecFromStr("0.896163650793650"),
		mustParse("2022-Jun-30"): numeric.MustNewDecFromStr("0.902834999999999"),
		mustParse("2022-Jul-31"): numeric.MustNewDecFromStr("0.909506349206348"),
		mustParse("2022-Aug-31"): numeric.MustNewDecFromStr("0.916177698412697"),
		mustParse("2022-Sep-30"): numeric.MustNewDecFromStr("0.922849047619047"),
		mustParse("2022-Oct-31"): numeric.MustNewDecFromStr("0.929520396825396"),
		mustParse("2022-Nov-30"): numeric.MustNewDecFromStr("0.936191746031745"),
		mustParse("2022-Dec-31"): numeric.MustNewDecFromStr("0.942863095238094"),
		// 2023
		mustParse("2023-Jan-31"): numeric.MustNewDecFromStr("0.944833333333332"),
		mustParse("2023-Feb-28"): numeric.MustNewDecFromStr("0.946803571428570"),
		mustParse("2023-Mar-31"): numeric.MustNewDecFromStr("0.948773809523808"),
		mustParse("2023-Apr-30"): numeric.MustNewDecFromStr("0.950744047619047"),
		mustParse("2023-May-31"): numeric.MustNewDecFromStr("0.952714285714285"),
		mustParse("2023-Jun-30"): numeric.MustNewDecFromStr("0.954684523809523"),
		mustParse("2023-Jul-31"): numeric.MustNewDecFromStr("0.956654761904761"),
		mustParse("2023-Aug-31"): numeric.MustNewDecFromStr("0.958624999999999"),
		mustParse("2023-Sep-30"): numeric.MustNewDecFromStr("0.960595238095237"),
		mustParse("2023-Oct-31"): numeric.MustNewDecFromStr("0.962565476190475"),
		mustParse("2023-Nov-30"): numeric.MustNewDecFromStr("0.964535714285713"),
		mustParse("2023-Dec-31"): numeric.MustNewDecFromStr("0.966505952380951"),
		// 2024
		mustParse("2024-Jan-31"): numeric.MustNewDecFromStr("0.968476190476189"),
		mustParse("2024-Feb-29"): numeric.MustNewDecFromStr("0.970446428571428"),
		mustParse("2024-Mar-31"): numeric.MustNewDecFromStr("0.972416666666666"),
		mustParse("2024-Apr-30"): numeric.MustNewDecFromStr("0.974386904761904"),
		mustParse("2024-May-31"): numeric.MustNewDecFromStr("0.976357142857142"),
		mustParse("2024-Jun-30"): numeric.MustNewDecFromStr("0.978327380952380"),
		mustParse("2024-Jul-31"): numeric.MustNewDecFromStr("0.980297619047618"),
		mustParse("2024-Aug-31"): numeric.MustNewDecFromStr("0.982267857142856"),
		mustParse("2024-Sep-30"): numeric.MustNewDecFromStr("0.984238095238094"),
		mustParse("2024-Oct-31"): numeric.MustNewDecFromStr("0.986208333333332"),
		mustParse("2024-Nov-30"): numeric.MustNewDecFromStr("0.988178571428570"),
		mustParse("2024-Dec-31"): numeric.MustNewDecFromStr("0.990148809523809"),
		// 2025
		mustParse("2025-Jan-31"): numeric.MustNewDecFromStr("0.992119047619047"),
		mustParse("2025-Feb-28"): numeric.MustNewDecFromStr("0.994089285714285"),
		mustParse("2025-Mar-31"): numeric.MustNewDecFromStr("0.996059523809523"),
		mustParse("2025-Apr-30"): numeric.MustNewDecFromStr("0.998029761904761"),
		mustParse("2025-May-31"): numeric.MustNewDecFromStr("1.000000000000000"),
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

	return bucket.share
}
