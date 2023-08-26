// cli is the module for customized cli structures for cobra command.
// The module is a wrapper over spf13/pflag, and serve to clean the code structure.

package cli

import (
	"github.com/spf13/pflag"
)

// Flag is the interface for cli flags.
// To get the value after cli parsing, use fs.GetString(flag.Name)
type Flag interface {
	RegisterTo(fs *pflag.FlagSet) error
}

// StringFlag is the flag with string value
type StringFlag struct {
	Name       string
	Shorthand  string
	Usage      string
	Deprecated string
	Hidden     bool

	DefValue string
}

// RegisterTo register the string flag to FlagSet
func (f StringFlag) RegisterTo(fs *pflag.FlagSet) error {
	fs.StringP(f.Name, f.Shorthand, f.DefValue, f.Usage)
	return markHiddenOrDeprecated(fs, f.Name, f.Deprecated, f.Hidden)
}

// BoolFlag is the flag with boolean value
type BoolFlag struct {
	Name       string
	Shorthand  string
	Usage      string
	Deprecated string
	Hidden     bool

	DefValue bool
}

// RegisterTo register the boolean flag to FlagSet
func (f BoolFlag) RegisterTo(fs *pflag.FlagSet) error {
	fs.BoolP(f.Name, f.Shorthand, f.DefValue, f.Usage)
	return markHiddenOrDeprecated(fs, f.Name, f.Deprecated, f.Hidden)
}

// IntFlag is the flag with int value
type IntFlag struct {
	Name       string
	Shorthand  string
	Usage      string
	Deprecated string
	Hidden     bool

	DefValue int
}

// RegisterTo register the int flag to FlagSet
func (f IntFlag) RegisterTo(fs *pflag.FlagSet) error {
	fs.IntP(f.Name, f.Shorthand, f.DefValue, f.Usage)
	return markHiddenOrDeprecated(fs, f.Name, f.Deprecated, f.Hidden)
}

// Int64Flag is the flag with int64 value, used for gwei configurations
type Int64Flag struct {
	Name       string
	Shorthand  string
	Usage      string
	Deprecated string
	Hidden     bool
	DefValue   int64
}

// RegisterTo register the int flag to FlagSet
func (f Int64Flag) RegisterTo(fs *pflag.FlagSet) error {
	fs.Int64P(f.Name, f.Shorthand, f.DefValue, f.Usage)
	return markHiddenOrDeprecated(fs, f.Name, f.Deprecated, f.Hidden)
}

// Uint64Flag is the flag with uint64 value, used for block number configurations
type Uint64Flag struct {
	Name       string
	Shorthand  string
	Usage      string
	Deprecated string
	Hidden     bool
	DefValue   uint64
}

// RegisterTo register the int flag to FlagSet
func (f Uint64Flag) RegisterTo(fs *pflag.FlagSet) error {
	fs.Uint64P(f.Name, f.Shorthand, f.DefValue, f.Usage)
	return markHiddenOrDeprecated(fs, f.Name, f.Deprecated, f.Hidden)
}

// StringSliceFlag is the flag with string slice value
type StringSliceFlag struct {
	Name       string
	Shorthand  string
	Usage      string
	Deprecated string
	Hidden     bool

	DefValue []string
}

// RegisterTo register the string slice flag to FlagSet
func (f StringSliceFlag) RegisterTo(fs *pflag.FlagSet) error {
	fs.StringSliceP(f.Name, f.Shorthand, f.DefValue, f.Usage)
	return markHiddenOrDeprecated(fs, f.Name, f.Deprecated, f.Hidden)
}

// IntSliceFlag is the flag with int slice value
type IntSliceFlag struct {
	Name       string
	Shorthand  string
	Usage      string
	Deprecated string
	Hidden     bool

	DefValue []int
}

// RegisterTo register the string slice flag to FlagSet
func (f IntSliceFlag) RegisterTo(fs *pflag.FlagSet) error {
	fs.IntSliceP(f.Name, f.Shorthand, f.DefValue, f.Usage)
	return markHiddenOrDeprecated(fs, f.Name, f.Deprecated, f.Hidden)
}

func markHiddenOrDeprecated(fs *pflag.FlagSet, name string, deprecated string, hidden bool) error {
	if len(deprecated) != 0 {
		// TODO: after totally removed node.sh, change MarkHidden to MarkDeprecated
		if err := fs.MarkHidden(name); err != nil {
			return err
		}
	} else if hidden {
		if err := fs.MarkHidden(name); err != nil {
			return err
		}
	}
	return nil
}

func getFlagName(flag Flag) string {
	switch f := flag.(type) {
	case StringFlag:
		return f.Name
	case IntFlag:
		return f.Name
	case BoolFlag:
		return f.Name
	case StringSliceFlag:
		return f.Name
	case IntSliceFlag:
		return f.Name
	case Int64Flag:
		return f.Name
	case Uint64Flag:
		return f.Name
	}
	return ""
}
