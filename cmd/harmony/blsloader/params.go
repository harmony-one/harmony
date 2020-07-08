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

// PassSrcType is the type of passphrase provider source.
// Three options available:
//  PassSrcFile - Read the passphrase from file
//  PassSrcPrompt - Read the passphrase from prompt
//  PassSrcPrompt - First try to unlock with passphrase from file, then read passphrase from prompt
type PassSrcType uint8

const (
	PassSrcNil    PassSrcType = iota // place holder for nil src
	PassSrcAuto                      // first try to unlock with pass from file, then look for prompt
	PassSrcFile                      // provide the passphrase through a pass file
	PassSrcPrompt                    // provide the passphrase through prompt
)

func (srcType PassSrcType) isValid() bool {
	switch srcType {
	case PassSrcAuto, PassSrcFile, PassSrcPrompt:
		return true
	default:
		return false
	}
}

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
