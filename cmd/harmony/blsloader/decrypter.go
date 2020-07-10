package blsloader

import bls_core "github.com/harmony-one/bls/ffi/go/bls"

type decrypter interface {
	validate() error
	decrypt(keyFile string) (*bls_core.SecretKey, error)
}
