package blsloader

import bls_core "github.com/harmony-one/bls/ffi/go/bls"

type decrypter interface {
	extension() string
	decrypt(keyFile string) (*bls_core.SecretKey, error)
}
