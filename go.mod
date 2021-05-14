module github.com/harmony-one/harmony

go 1.16

require (
	github.com/Workiva/go-datastructures v1.0.53
	github.com/aws/aws-sdk-go v1.38.39
	github.com/beevik/ntp v0.3.0
	github.com/btcsuite/btcutil v1.0.2
	github.com/cespare/cp v1.1.1
	github.com/coinbase/rosetta-sdk-go v0.4.6
	github.com/davecgh/go-spew v1.1.1
	github.com/deckarep/golang-set v1.7.1
	github.com/ethereum/go-ethereum v1.9.25
	github.com/garslo/gogen v0.0.0-20170307003452-d6ebae628c7c // indirect
	github.com/golang/mock v1.5.0
	github.com/golang/protobuf v1.5.2
	github.com/golangci/golangci-lint v1.40.0
	github.com/gorilla/mux v1.8.0
	github.com/harmony-ek/gencodec v0.0.0-20190215044613-e6740dbdd846
	github.com/harmony-one/abool v1.0.1
	github.com/harmony-one/bls v0.0.6
	github.com/harmony-one/taggedrlp v0.1.4
	github.com/harmony-one/vdf v1.0.0
	github.com/hashicorp/go-version v1.3.0
	github.com/hashicorp/golang-lru v0.5.5-0.20210104140557-80c98217689d
	github.com/ipfs/go-ds-badger v0.2.6
	github.com/libp2p/go-libp2p v0.14.0
	github.com/libp2p/go-libp2p-core v0.8.5
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-kad-dht v0.12.0
	github.com/libp2p/go-libp2p-pubsub v0.4.1
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/pborman/uuid v1.2.1
	github.com/pelletier/go-toml v1.9.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.10.0
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/rjeczalik/notify v0.9.2
	github.com/rs/zerolog v1.22.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20210305035536-64b5b1c73954
	go.uber.org/ratelimit v0.2.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/tools v0.1.1
	google.golang.org/grpc v1.37.1
	google.golang.org/protobuf v1.26.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/olebedev/go-duktape.v3 v3.0.0-20210326210528-650f7c854440
	gopkg.in/yaml.v2 v2.4.0
)

replace github.com/ethereum/go-ethereum => github.com/ethereum/go-ethereum v1.9.9
