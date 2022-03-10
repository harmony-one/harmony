module github.com/harmony-one/harmony

go 1.16

require (
	github.com/VictoriaMetrics/fastcache v1.5.7 // indirect
	github.com/Workiva/go-datastructures v1.0.50
	github.com/allegro/bigcache v1.2.1
	github.com/aristanetworks/goarista v0.0.0-20190607111240-52c2a7864a08 // indirect
	github.com/aws/aws-sdk-go v1.30.1
	github.com/beevik/ntp v0.3.0
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce
	github.com/cespare/cp v1.1.1
	github.com/coinbase/rosetta-sdk-go v0.7.0
	github.com/davecgh/go-spew v1.1.1
	github.com/deckarep/golang-set v1.7.1
	github.com/ethereum/go-ethereum v1.9.25
	github.com/fjl/memsize v0.0.0-20180929194037-2a09253e352a // indirect
	github.com/garslo/gogen v0.0.0-20170307003452-d6ebae628c7c // indirect
	github.com/golang/mock v1.6.0
	github.com/golang/protobuf v1.5.2
	github.com/golangci/golangci-lint v1.22.2
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/harmony-ek/gencodec v0.0.0-20190215044613-e6740dbdd846
	github.com/harmony-one/abool v1.0.1
	github.com/harmony-one/bls v0.0.6
	github.com/harmony-one/taggedrlp v0.1.4
	github.com/harmony-one/vdf v0.0.0-20190924175951-620379da8849
	github.com/hashicorp/go-version v1.2.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-ds-badger v0.2.7
	github.com/json-iterator/go v1.1.10
	github.com/libp2p/go-libp2p v0.14.4
	github.com/libp2p/go-libp2p-core v0.8.6
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/libp2p/go-libp2p-discovery v0.5.1
	github.com/libp2p/go-libp2p-kad-dht v0.11.1
	github.com/libp2p/go-libp2p-pubsub v0.5.6
	github.com/multiformats/go-multiaddr v0.3.3
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/pborman/uuid v1.2.0
	github.com/pelletier/go-toml v1.9.3
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/rjeczalik/notify v0.9.2
	github.com/rs/cors v1.7.0
	github.com/rs/zerolog v1.18.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.1
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20200815110645-5c35d600f0ca
	github.com/whyrusleeping/timecache v0.0.0-20160911033111-cfcb2f1abfee // indirect
	go.uber.org/ratelimit v0.1.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20210506145944-38f3c27a63bf
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/time v0.0.0-20210220033141-f8bda1e9f3ba
	golang.org/x/tools v0.1.7
	google.golang.org/grpc v1.33.2
	google.golang.org/protobuf v1.26.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/natefinch/npipe.v2 v2.0.0-20160621034901-c1b8fa8bdcce
	gopkg.in/olebedev/go-duktape.v3 v3.0.0-20200619000410-60c24ae608a6
	gopkg.in/yaml.v2 v2.4.0
	honnef.co/go/tools v0.0.1-2020.1.5 // indirect
)

replace github.com/ethereum/go-ethereum => github.com/ethereum/go-ethereum v1.9.9
