package main

const (
	defaultWalletIni = `[default]
bootnode = /ip4/100.26.90.187/tcp/9876/p2p/QmZJJx6AdaoEkGLrYG4JeLCKeCKDjnFz2wfHNHxAqFSGA9
bootnode = /ip4/54.213.43.194/tcp/9876/p2p/QmQayinFSgMMw5cSpDUiD9pQ2WeP6WNmGxpZ6ou3mdVFJX
shards = 1

[default.shard0.rpc]
rpc = 34.217.179.222:14555
rpc = 18.209.247.105:14555
rpc = 100.25.248.42:14555
rpc = 3.80.164.193:14555
rpc = 54.87.237.93:14555

[local]
bootnode = /ip4/127.0.0.1/tcp/19876/p2p/Qmc1V6W7BwX8Ugb42Ti8RnXF1rY5PF7nnZ6bKBryCgi6cv
shards = 1

[local.shard0.rpc]
rpc = 127.0.0.1:14555
rpc = 127.0.0.1:14556

[devnet]
bootnode = /ip4/100.26.90.187/tcp/9871/p2p/Qmdfjtk6hPoyrH1zVD9PEH4zfWLo38dP2mDvvKXfh3tnEv
bootnode = /ip4/54.213.43.194/tcp/9871/p2p/QmRVbTpEYup8dSaURZfF6ByrMTSKa4UyUzJhSjahFzRqNj
shards = 3

[devnet.shard0.rpc]
rpc = 13.57.196.136:14555
rpc = 35.175.103.144:14555
rpc = 54.245.176.36:14555

[devnet.shard1.rpc]
rpc = 35.163.188.234:14555
rpc = 54.215.251.123:14555
rpc = 54.153.11.146:14555

[devnet.shard2.rpc]
rpc = 52.201.246.212:14555
rpc = 3.81.26.139:14555
rpc = 18.237.42.209:14555
`
)
