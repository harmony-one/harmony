package blsloader

import "time"

const (
	passExt     = ".pass"
	basicKeyExt = ".key"
	kmsKeyExt   = ".bls"
)

const (
	defKmsPromptTimeout  = 1 * time.Second
	defPassPromptTimeout = 10 * time.Second
)

const (
	defWritePassDirMode  = 0600
	defWritePassFileMode = 0600
)
