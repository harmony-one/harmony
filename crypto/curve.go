package crypto

import "github.com/dedis/kyber/group/edwards25519"

// Ed25519Curve value gets initialized.
var Ed25519Curve = edwards25519.NewBlakeSHA256Ed25519()
