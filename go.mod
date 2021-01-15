module github.com/harmony-one/harmony

go 1.14

require (
	github.com/VictoriaMetrics/fastcache v1.5.7 // indirect
	github.com/Workiva/go-datastructures v1.0.50
	github.com/allegro/bigcache v1.2.1 // indirect
	github.com/aristanetworks/goarista v0.0.0-20190607111240-52c2a7864a08 // indirect
	github.com/aws/aws-sdk-go v1.30.1
	github.com/beevik/ntp v0.3.0
	github.com/btcsuite/btcutil v1.0.2
	github.com/cespare/cp v1.1.1
	github.com/coinbase/rosetta-sdk-go v0.4.6
	github.com/davecgh/go-spew v1.1.1
	github.com/deckarep/golang-set v1.7.1
	github.com/edsrzf/mmap-go v1.0.0 // indirect
	github.com/ethereum/go-ethereum v1.9.21
	github.com/fjl/memsize v0.0.0-20180929194037-2a09253e352a // indirect
	github.com/garslo/gogen v0.0.0-20170307003452-d6ebae628c7c // indirect
	github.com/golang/gddo v0.0.0-20200831202555-721e228c7686
	github.com/golang/mock v1.4.0
	github.com/golang/protobuf v1.4.3
	github.com/golang/snappy v0.0.2-0.20200707131729-196ae77b8a26 // indirect
	github.com/golangci/golangci-lint v1.22.2
	github.com/gorilla/mux v1.8.0
	github.com/harmony-ek/gencodec v0.0.0-20190215044613-e6740dbdd846
	github.com/harmony-one/abool v1.0.1
	github.com/harmony-one/bls v0.0.6
	github.com/harmony-one/taggedrlp v0.1.4
	github.com/harmony-one/vdf v0.0.0-20190924175951-620379da8849
	github.com/hashicorp/golang-lru v0.5.4
	github.com/ipfs/go-ds-badger v0.2.4
	github.com/libp2p/go-libp2p v0.13.0
	github.com/libp2p/go-libp2p-core v0.8.0
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/libp2p/go-libp2p-discovery v0.5.0
	github.com/libp2p/go-libp2p-kad-dht v0.11.1
	github.com/libp2p/go-libp2p-pubsub v0.4.0
	github.com/multiformats/go-multiaddr v0.3.1
	github.com/multiformats/go-multiaddr-dns v0.2.0
	github.com/multiformats/go-multiaddr-net v0.2.0
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/pborman/uuid v1.2.0
	github.com/pelletier/go-toml v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.8.0
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/rjeczalik/notify v0.9.2
	github.com/rs/cors v1.7.0 // indirect
	github.com/rs/zerolog v1.18.0
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.6.1
	github.com/stretchr/testify v1.6.1
	github.com/syndtr/goleveldb v1.0.1-0.20200815110645-5c35d600f0ca
	golang.org/x/crypto v0.0.0-20200820211705-5c72a883971a
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/tools v0.0.0-20200904185747-39188db58858
	google.golang.org/grpc v1.31.1
	google.golang.org/protobuf v1.25.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/olebedev/go-duktape.v3 v3.0.0-20190213234257-ec84240a7772
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/ethereum/go-ethereum => github.com/ethereum/go-ethereum v1.9.9
