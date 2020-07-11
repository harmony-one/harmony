package blsloader

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
	defKmsPromptTimeout = 1 * time.Second
)

const (
	defWritePassDirMode  = 0700
	defWritePassFileMode = 0600
)
