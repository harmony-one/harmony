package node

import (
	"time"

	"github.com/harmony-one/harmony/internal/rate"

	libp2p_pubsub "github.com/libp2p/go-libp2p-pubsub"
)

const (
	// TODO: make these parameters configurable
	pubSubRateLimit  = 4
	pubSubBurstLimit = 40

	// When a node is bootstrapped, it will be flooded with some pub-sub message from the past.
	// Ease the rate limit at the initial bootstrap.
	rateLimiterEasyPeriod = 30 * time.Second

	blacklistExpiry = 5 * time.Minute
)

var (
	// TODO: Add trusted peer id here (internal explorer, leader candidates, e.t.c)
	// TODO: make these parameters configurable
	whitelist = []string{
		"QmZr6bcTVipdjfSEWz46UrKeWjcsT3BP9KMZzq3CNAfA7Z",
		"QmbNp5DeJwSh4fcSJDAS295yYyp3NfWdr7nmTWPBA4BYYR",
		"QmSVjgFUKiSdLd6Nx8XHHmEfSSstb24bdpbCXdd41cC6xk",
		"QmSW8U8FqMWjnU8g5iejgr8ZWmpCewvLvnKTmfMcr9LvbY",
		"QmUAnJmZxKotagK6KqhMGWaU1nETJdFfeYoP5NeSP3xaAM",
		"Qmdq76KDgiPDrPT6DwLHvhyePKnXCBcVGVjUQbiwBYxSZ6",
		"QmTK3A24bZRnfuzUiGK9QY7CNRx2DM8bf6TyrbSgT789Xc",
		"QmYU4zWjVcFTcHH9xUyhb3RYGKii5iuTNAz2a4oskogM1R",
		"QmYpNxgPSm8qr11yJDL6Gy7JdPJoKfGz4GzBNTvDTYnMPP",
		"QmVgJMSgiJqCZzdMJQBQTLYvJVeEEUTr57tSmd3oT91myd",
		"QmNgJrFWFkgAUZ4x92TtZY4oPJQLnDjHesc5gEnXzYF1ti",
		"QmSangD1c1iSGAeHemixo5jaaahLVtq9NMVXZQxpstaPKi",
		"QmTSqgq5YHbG1gCx5C2dWi9Qj9rd7rVhbSXACJYVJbscsY",
		"QmWQ7cD3vRtEyQbD9dvZeEMfdESCxeK6cJmzNYGqMeLuqx",
		"QmYoYFNynJVa8mYwm8vVML3DgstNnNF9TaB99nee2SoPLs",
		"Qma3bAUaejJvHsEosVZBGrzF4mErVY5xzsTMZn5mmwUSPi",
		"QmNaW9RL57bi6kfKRqovQKYxx72nYxrKVXcspEvMRPChfR",
		"QmNbPKsG34pgE6JRoK7cX4Vbp5jsKganQkSS4wnXAgBhPn",
		"QmZFYgKS7mipHW25FpGf4Tfwhsrq8srmDKHPWtx4ahAFW6",
		"QmVo9ArXB8W6BGDF6c8fkXtvBrDK5ZM1Cmy5W1Y9w3VGZ2",
		"QmR8NnQrjPkUDZK5EiooTif4rvWjqX42VZEcZ9Hc7dgmc5",
		"Qmc1L8xC2mDjJxUdMhGJRfFti9NvX1AjZVV2wqtSWEnHdL",
		"QmPx2DcjAnBPndHU9pwXDeVUWseT4GgVE62fdD1GevuRrm",
		"QmWaVpLP4mLaBGH7W9fbWvR4UV8TpAg1UpRU3Fy5J11zm4",
		"QmS3fEWDBJUiAYX8VHmnyZb5sRNVWtUhKxmJMPour8ku19",
		"QmQovqUu5sdUZPqdRnDiEzMWh84wFP2CNRMqKVCDSceqj2",
		"QmWibkR2oaVENDvPhSWeYKDkqEthe1XcokmN2HeU6vSfsZ",
		"QmWr3yHHDoD3gSfWLNNTA6KXdhwLy2oe6UrcQjx6UCLu7x",
		"QmauxE3ntFGhuq3wbhvEt4ZTpVbt2EcW2Z5k6rcCZncVYn",
		"QmYXSF22bNvt9wihZbKsyrNvGGm3yAzqivf7SFVMaQjBam",
		"QmP86P37MNMN4nT7NsKKvAWzGAhCnMd8ohddipZmh4V7Li",
		"QmWW2fNTZitNqfXcPjDhPWCYN94KrTttqkr5xSo4r4u6uq",
		"QmbKweCSVkAnu1tm1wGgvzqByw7UP4tSiePSuwkWD5ywGS",
		"QmPvSUgNV2mVhGTYTMHaBbiyqYQMo6DGs5qrdocM3gj4gg",
		"QmPvSUgNV2mVhGTYTMHaBbiyqYQMo6DGs5qrdocM3gj4gg",
		"QmbEkvygjXunktCLCKyJ2tmhwRrVAMo97bD2fP4iuL3bGA",
		"QmYrm1PNmUFHTH6vKkihh4eTKQhYF8FUTBmBmQeGf8en67",
		"QmUm2V6o7m4x9nSB1mYunM7ZWRqHSMBpK4fe3dfUwFsf87",
		"QmToQEQPmvCm3M79u6Gu6pDtQvFto9oL3AWUcwLba5qEUz",
		"QmXdaVjUxQk39fA1QP5Qfg9FnkGX69MT3Saizc4QEFhgfS",
		"QmXzDVUPQ5wnsYQ7ZTKt8QxM7zQGrjsedPhtfb98CQRJ5m",
		"QmYoqAr91r9ib98ZxJd1ubu5KvwgavHWJg9sSDVgG41DsT",
		"QmZRpXswsGkfuxSAPaC6JfznRVxHkwbQjKLfsWTJDmSri7",
		"QmbXY5fs1XC9gSgo43WVFoyGMQoRzye5QvZ2H7zcDrUHt2",
		"QmWfytuUA4kBjfcZnWnxtPKCSPFEYt7oc3wSBDU333RF3t",
		"QmcpyWGKJ7Fc6cvvDKWnj9APL2MXTUc3Sz93T3zvyfT4E6",
		"QmXTqQWrvzTXRx14CD7GD1ukJuc1Df6nzViagWWB67WWAU",
		"QmZXkSFPbDyLySjyjCEJpHYjCQkhMqstfYcKSjSHE1LpX4",
		"QmahtfauneHxCZh6PowLXfYUqmQwdsPKHmTvf7bBtWmHvb",
		"QmfJDTiPHsBESZN1LBKFHhJ1DbuLNVJczfRLwZna7A2eP5",
		"QmPE8wgwiimy6d9yiaaafC1kKNo6iPBcG1EUK6VyrMaobA",
		"QmRGAwQQRoKwQKuhEL2m8K2ECjgBB7jXNfAnqqzstvuJoa",
		"QmcWxoHkkPN1Wv2jQMKJuCWxwGEWpMuRVhVm4DLQHvf3TR",
		"QmWQUBHo4b3EAZFYbPoUoDFqWhDxWDV6KD8gRJHZBVLk7y",
		"QmZmWVk8cXhKes6wi8eD8sf7WwE7A9Y1VzeiavcEiKGMdk",
		"QmdEc7hpoPnpW2RQfr1o77yD29HmSozZWkCtzsjTAPnLSN",
		"QmTDqofFA71ZgauinC23CpYkAsRcGru3tjDHJX5qFE7Yje",
		"QmVsUD4YEXwFgJw8BGrd5eoEQbM1PeYHwAq83mVLqgPFah",
		"QmVfQaiWXaHWQ8XxBG4wbrYRbV6bLNVpFYRZpdMj2BW5Ew",
		"QmPeevwngRVpRnxKdQbKJbuboAuphrkmk51HrudZ7p31fN",
		"QmYZHAVEVfVjJMXGHhc94mr5Kfi5kvAoRJfoQJN1bHUaWm",
		"QmTByGyQ8NM27rKHP5TkPDD4iNjd6bZWoCQTbjp6epsoWK",
		"QmNpnLLo3txr7VAicDxtrSAQ2bqxPHZzsktRpzPAVMwuoE",
		"QmSthXVRGxkgMVp4togAuF1Mv8432GQmyPUps81o3CZDjm",
		"QmVCoU2KYSzBmUGLJ91KzbiSjMADPfVjM8Rghdnm4RMvvE",
		"QmWEsUY5dFy42vQhFYfXRQ4wUmC1YdDJJDiTS2Kct5q2XP",
		"QmPJogzgqCLuG219rj8Zt96TvyieP1nDyFdVJsA6ChHNrd",
		"QmbDMWMPRdCowa8tDWgA3JpuEJAeHCeshqW5EjjV67LeMp",
		"QmbN2SyHQaiHD1PFY5fgbGP3S2yRpaEMjcisV2kBfAwra9",
		"QmWgEvXcwSsAyBCpx2GGd9N2XqJWnJsxR4dKjwvHCFmkp9",
		"QmdezDtLQtZy16ft8SHdskBAYJcKjiiTxvbqvzABeGgXXp",
		"QmXiakUJja4w3FidGnvz833NgPThEt7LLc6jujqXhzC1GY",
		"QmYP2SbWdgZCJneo77jZrKQaaKdzUy8vKUVKb28BRsL6ca",
		"QmduutehJyioLnutsJJH5ZBctGqMUkMbdieKN81ETcDLwe",
		"QmfKvoTuNW5YnBVND63nAaW7wJ4QQEyhpNytTCQaYsDjeS",
		"QmdyMYiq1V3EEmZwqno2uBTn5UJq5kJmrSFH6AGzs8yyCh",
		"QmNuybXASbaMLHo8CoyiySiCEm8Cs5VAYwyGRBbN4zse7F",
		"QmY3M9JNBVho9r1hqUEH3CNghq2XPLfNC93Pqm8niaNg25",
		"Qmdm2NAexi1KMBNVz5WsEtBtRQUK6dSAuJRjMfNLuCi4Ft",
		"QmdFbP3M2pjdwpVK4mTpZrnPKwR2QEuJ6Z9HodNxCAJaKD",
		"QmfHuuJ91cW5uUCKpwfZrZimrMjRxNPkeowsDYWoghfFmx",
		"QmSGGt6DRcaJutDci7hP7fmSsgMm61ZbrWnimt7qezpEes",
		"QmPffmqBHnY3tnG6parVdEwSVLtKyhgX4jf6pBgtDFyJrZ",
		"QmVRcZM8yK2XPj5F9cnkgYrRHHjednZXRm3mym598Ady1d",
		"QmRqRnZAsc6kd7LJCTrTTMTCeEMxzmySWdYpktVMYkN8SZ",
		"Qmbpahmoy7ELaA3efG1Cd1sba1T4hrGQqUb5mehLV7HgQz",
		"QmXhYD6uVTsCLJq44nJSEBkEHwNHCKoowA3NL9RtYZqigz",
		"QmbPhBqqv7UsSCAgEre7FR2DWgmu3ruc2mmuANdjN2ftEF",
		"QmU26vm1sgZJdfxYaWwmx5pbqHwqBWeGueF6yzv5VkzRkp",
		"QmWPgidDQpE8teJtqp7N56bj6HttJgvph4C39UVrtkqRTd",
		"QmNkbrtbyJtFZtfdumkJkSzK9oiY1Xu3vVpBMyxXLxKhkp",
		"QmQ7fWBuqAmtaG2pozE2n5rPd6bBdvhhneTfeFSbsMSTY9",
		"QmbSFi1zGwyxJ9VuFuzNbvvrytHqMaxyrR6ha5TPmZxWE9",
	}
)

func newBlackList() libp2p_pubsub.Blacklist {
	blacklist, _ := libp2p_pubsub.NewTimeCachedBlacklist(blacklistExpiry)
	return blacklist
}

func newRateLimiter() rate.IDLimiter {
	return rate.NewLimiterPerID(pubSubRateLimit, pubSubBurstLimit, &rate.Config{
		Whitelist: whitelist,
	})
}
