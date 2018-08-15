package crypto

import "crypto/sha256"

func HashSha256(message string) [32]byte {
	return sha256.Sum256([]byte(message))
}
