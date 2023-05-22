package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type errorHandle func(error)

var (
	parseErrorHandleFunc errorHandle
)

// SetParseErrorHandle set the error handle function used for cli parsing flags.
// An error handle example:
//
//	cli.SetParseErrorHandle(func(err error) {
//		fmt.Println(err)
//		os.Exit(3)
//	})
func SetParseErrorHandle(f errorHandle) {
	parseErrorHandleFunc = f
}

// GetStringFlagValue get the string value for the given StringFlag from the local flags
// of the cobra command.
func GetStringFlagValue(cmd *cobra.Command, flag StringFlag) string {
	return getStringFlagValue(cmd.Flags(), flag)
}

// GetStringPersistentFlagValue get the string value for the given StringFlag from the persistent
// flags of the cobra command.
func GetStringPersistentFlagValue(cmd *cobra.Command, flag StringFlag) string {
	return getStringFlagValue(cmd.PersistentFlags(), flag)
}

func getStringFlagValue(fs *pflag.FlagSet, flag StringFlag) string {
	val, err := fs.GetString(flag.Name)
	if err != nil {
		handleParseError(err)
		return ""
	}
	return val
}

// GetBoolFlagValue get the bool value for the given BoolFlag from the local flags of the
// cobra command.
func GetBoolFlagValue(cmd *cobra.Command, flag BoolFlag) bool {
	return getBoolFlagValue(cmd.Flags(), flag)
}

// GetBoolPersistentFlagValue get the bool value for the given BoolFlag from the persistent flags
// of the given cobra command.
func GetBoolPersistentFlagValue(cmd *cobra.Command, flag BoolFlag) bool {
	return getBoolFlagValue(cmd.PersistentFlags(), flag)
}

func getBoolFlagValue(fs *pflag.FlagSet, flag BoolFlag) bool {
	val, err := fs.GetBool(flag.Name)
	if err != nil {
		handleParseError(err)
		return false
	}
	return val
}

// GetIntFlagValue get the int value for the given IntFlag from the local flags of the
// cobra command.
func GetIntFlagValue(cmd *cobra.Command, flag IntFlag) int {
	return getIntFlagValue(cmd.Flags(), flag)
}

// GetInt64FlagValue get the int value for the given Int64Flag from the local flags of the
// cobra command.
func GetInt64FlagValue(cmd *cobra.Command, flag Int64Flag) int64 {
	return getInt64FlagValue(cmd.Flags(), flag)
}

// GetIntPersistentFlagValue get the int value for the given IntFlag from the persistent
// flags of the cobra command.
func GetIntPersistentFlagValue(cmd *cobra.Command, flag IntFlag) int {
	return getIntFlagValue(cmd.PersistentFlags(), flag)
}

func getIntFlagValue(fs *pflag.FlagSet, flag IntFlag) int {
	val, err := fs.GetInt(flag.Name)
	if err != nil {
		handleParseError(err)
		return 0
	}
	return val
}

func getInt64FlagValue(fs *pflag.FlagSet, flag Int64Flag) int64 {
	val, err := fs.GetInt64(flag.Name)
	if err != nil {
		handleParseError(err)
		return 0
	}
	return val
}

// GetStringSliceFlagValue get the string slice value for the given StringSliceFlag from
// the local flags of the cobra command.
func GetStringSliceFlagValue(cmd *cobra.Command, flag StringSliceFlag) []string {
	return getStringSliceFlagValue(cmd.Flags(), flag)
}

// GetStringSlicePersistentFlagValue get the string slice value for the given StringSliceFlag
// from the persistent flags of the cobra command.
func GetStringSlicePersistentFlagValue(cmd *cobra.Command, flag StringSliceFlag) []string {
	return getStringSliceFlagValue(cmd.PersistentFlags(), flag)
}

func getStringSliceFlagValue(fs *pflag.FlagSet, flag StringSliceFlag) []string {
	val, err := fs.GetStringSlice(flag.Name)
	if err != nil {
		handleParseError(err)
		return nil
	}
	return val
}

// GetIntSliceFlagValue get the int slice value for the given IntSliceFlag from
// the local flags of the cobra command.
func GetIntSliceFlagValue(cmd *cobra.Command, flag IntSliceFlag) []int {
	return getIntSliceFlagValue(cmd.Flags(), flag)
}

// GetIntSlicePersistentFlagValue get the int slice value for the given IntSliceFlag
// from the persistent flags of the cobra command.
func GetIntSlicePersistentFlagValue(cmd *cobra.Command, flag IntSliceFlag) []int {
	return getIntSliceFlagValue(cmd.PersistentFlags(), flag)
}

func getIntSliceFlagValue(fs *pflag.FlagSet, flag IntSliceFlag) []int {
	val, err := fs.GetIntSlice(flag.Name)
	if err != nil {
		handleParseError(err)
		return nil
	}
	return val
}

// IsFlagChanged returns whether the flag has been changed in command
func IsFlagChanged(cmd *cobra.Command, flag Flag) bool {
	name := getFlagName(flag)
	return cmd.Flags().Changed(name)
}

// HasFlagsChanged returns whether any of the flags is set by user in the command
func HasFlagsChanged(cmd *cobra.Command, flags []Flag) bool {
	fs := cmd.Flags()

	for _, flag := range flags {
		name := getFlagName(flag)
		if fs.Changed(name) {
			return true
		}
	}
	return false
}

func handleParseError(err error) {
	if parseErrorHandleFunc != nil {
		parseErrorHandleFunc(err)
	}
}
