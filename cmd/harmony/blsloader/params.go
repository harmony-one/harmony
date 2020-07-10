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
	defWritePassDirMode  = 0600
	defWritePassFileMode = 0600
)

// AwsConfigSrcType is the type of src to load aws config. Two options available
//  AwsCfgSrcFile - Provide the aws config through a file (json).
//  AwsCfgSrcPrompt - Provide the aws config though prompt.
//  AwsCfgSrcShared - Use the shard aws config (env -> default .aws directory)
type AwsCfgSrcType uint8

const (
	AwsCfgSrcNil    AwsCfgSrcType = iota // nil place holder.
	AwsCfgSrcFile                        // through a config file (json)
	AwsCfgSrcPrompt                      // through console prompt.
	AwsCfgSrcShared                      // through shared aws config
)

func (srcType AwsCfgSrcType) isValid() bool {
	switch srcType {
	case AwsCfgSrcFile, AwsCfgSrcPrompt, AwsCfgSrcShared:
		return true
	default:
		return false
	}
}
