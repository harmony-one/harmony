package blsloader

import "time"

const (
	passExt     = ".pass"
	basicKeyExt = ".key"
	kmsKeyExt   = ".bls"
)

const (
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
	PassSrcNil    PassSrcType = iota // nil place holder
	PassSrcFile                      // provide the passphrase through a pass file
	PassSrcPrompt                    // provide the passphrase through prompt
	PassSrcAuto                      // first try to unlock with pass from file, then look for prompt
)

func (srcType PassSrcType) String() string {
	switch srcType {
	case PassSrcFile:
		return "file"
	case PassSrcPrompt:
		return "prompt"
	case PassSrcAuto:
		return "auto"
	default:
		return "unknown"
	}
}

// PassSrcTypeFromString parse PassSrcType from string specifier
func PassSrcTypeFromString(str string) PassSrcType {
	switch str {
	case "file":
		return PassSrcFile
	case "prompt":
		return PassSrcPrompt
	case "auto":
		return PassSrcAuto
	default:
		return PassSrcNil
	}
}

// AwsConfigSrcType is the type of src to load aws config. Two options available
//  AwsCfgSrcFile - Provide the aws config through a file (customized or shard .aws dir).
//  AwsCfgSrcPrompt - Provide the aws config though prompt.
type AwsCfgSrcType uint8

const (
	AwsCfgSrcNil    AwsCfgSrcType = iota // nil place holder.
	AwsCfgSrcFile                        // through a config file (json)
	AwsCfgSrcPrompt                      // through console prompt.
	AwsCfgSrcShared                      // through shared aws config
)

func (srcType AwsCfgSrcType) String() string {
	switch srcType {
	case AwsCfgSrcFile:
		return "file"
	case AwsCfgSrcPrompt:
		return "prompt"
	case AwsCfgSrcShared:
		return "shared"
	default:
		return "unknown"
	}
}

func AwsCfgSrcTypeFromString(str string) AwsCfgSrcType {
	switch str {
	case "file":
		return AwsCfgSrcFile
	case "prompt":
		return AwsCfgSrcPrompt
	case "shared":
		return AwsCfgSrcShared
	default:
		return AwsCfgSrcNil
	}
}
