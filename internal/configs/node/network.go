package nodeconfig

var (
	mainnetBootNodes = []string{
		"/ip4/100.26.90.187/tcp/9874/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
		"/ip4/54.213.43.194/tcp/9874/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9",
		"/ip4/13.113.101.219/tcp/12019/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX",
		"/ip4/99.81.170.167/tcp/12019/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj",
	}

	testnetBootNodes = []string{
		"/ip4/54.86.126.90/tcp/9867/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
		"/ip4/52.40.84.2/tcp/9867/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29",
	}

	pangaeaBootNodes = []string{
		"/ip4/52.40.84.2/tcp/9800/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29",
		"/ip4/54.86.126.90/tcp/9800/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
	}

	partnerBootNodes = []string{
		"/ip4/52.40.84.2/tcp/9800/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29",
		"/ip4/54.86.126.90/tcp/9800/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv",
	}

	stressBootNodes = []string{
		"/ip4/52.40.84.2/tcp/9842/p2p/QmbPVwrqWsTYXq1RxGWcxx9SWaTUCfoo1wA6wmdbduWe29",
	}
)

const (
	mainnetDNSZone   = "t.hmny.io"
	testnetDNSZone   = "b.hmny.io"
	pangaeaDNSZone   = "os.hmny.io"
	partnerDNSZone   = "ps.hmny.io"
	stressnetDNSZone = "stn.hmny.io"
)

const (
	defaultDNSPort = 9000
)

// GetDefaultBootNodes get the default bootnode with the given network type
func GetDefaultBootNodes(networkType NetworkType) []string {
	switch networkType {
	case Mainnet:
		return mainnetBootNodes
	case Testnet:
		return testnetBootNodes
	case Pangaea:
		return pangaeaBootNodes
	case Partner:
		return partnerBootNodes
	case Stressnet:
		return stressBootNodes
	}
	return nil
}

// GetDefaultDNSZone get the default DNS zone with the given network type
func GetDefaultDNSZone(networkType NetworkType) string {
	switch networkType {
	case Mainnet:
		return mainnetDNSZone
	case Testnet:
		return testnetDNSZone
	case Pangaea:
		return pangaeaDNSZone
	case Partner:
		return partnerDNSZone
	case Stressnet:
		return stressnetDNSZone
	}
}

// GetDefaultDNSPort get the default DNS port for the given network type
func GetDefaultDNSPort(NetworkType) int {
	return defaultDNSPort
}
