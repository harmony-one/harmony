package crypto

import "github.com/dedis/kyber/group/edwards25519"

var Ed25519Curve = edwards25519.NewBlakeSHA256Ed25519()
