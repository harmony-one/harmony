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
// 		cli.SetParseErrorHandle(func(err error) {
//			fmt.Println(err)
// 			os.Exit(3)
// 		})
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

func handleParseError(err error) {
	if parseErrorHandleFunc != nil {
		parseErrorHandleFunc(err)
	}
}
