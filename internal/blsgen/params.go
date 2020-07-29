package blsgen

import "time"

const (
	// Extensions for files.
	passExt     = ".pass"
	basicKeyExt = ".key"
	kmsKeyExt   = ".bls"
)

const (
	// The default timeout for kms config prompt. The timeout is introduced
	// for security concern.
	defKmsPromptTimeout = 60 * time.Second
)

const (
	defWritePassFileMode = 0600
)
