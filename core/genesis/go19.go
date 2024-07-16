//go:build 1.19
// +build 1.19

package genesis

var ContractDeployerKey *ecdsa.PrivateKey

func init() {
	var err error
	ContractDeployerKey, err = ecdsa.GenerateKey(
		crypto.S256(),
		strings.NewReader("Test contract key string stream that is fixed so that generated test key are deterministic every time"),
	)
	if err != nil {
		panic(err)
	}
}
