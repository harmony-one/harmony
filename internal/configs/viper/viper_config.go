package viperconfig

import (
	"bytes"
	"fmt"

	"github.com/spf13/viper"
)

// CreateEnvViper creates viper to read variables from system enviroment
func CreateEnvViper() *viper.Viper {
	envViper := viper.New()
	envViper.SetEnvPrefix("HMY") // will be uppercased automatically
	envViper.AutomaticEnv()

	return envViper
}

// CreateConfFileViper creates viper to read from config file
// Now the config file is JSON type, name is "config.json"
func CreateConfFileViper(filePath, confName, confType string) *viper.Viper {
	configFileViper := viper.New()
	configFileViper.SetConfigName(confName) // name of config file (without extension)
	configFileViper.SetConfigType(confType) // REQUIRED if the config file does not have the extension in the name
	configFileViper.AddConfigPath(filePath) // look for the file in filePath
	configFileViper.AddConfigPath(".")      // or look for config in the working directory

	if err := configFileViper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(fmt.Errorf("fatal error config file: %s", err))
		}
	}
	return configFileViper
}

func getEnvName(sectionName string, flagName string) string {
	var buffer bytes.Buffer
	if sectionName != "" {
		buffer.WriteString("_")
		buffer.WriteString(sectionName)
	}
	buffer.WriteString("_")
	buffer.WriteString(flagName)
	return buffer.String()
}

func getConfName(sectionName string, flagName string) string {
	var buffer bytes.Buffer
	if sectionName != "" {
		buffer.WriteString(sectionName)
		buffer.WriteString(".")
	}
	buffer.WriteString(flagName)
	return buffer.String()
}

// ResetConfUInt resets UInt value to value from config files and system environment variable
func ResetConfUInt(value *uint, envViper *viper.Viper, configFileViper *viper.Viper, sectionName string, flagName string) {
	var confRet = configFileViper.GetInt(getConfName(sectionName, flagName))
	if confRet != 0 {
		*value = uint(confRet)
		return
	}

	var envRet = envViper.GetInt(getEnvName(sectionName, flagName))
	if envRet != 0 {
		*value = uint(envRet)
		return
	}
}

// ResetConfInt resets INT value to value from config files and system environment variable
func ResetConfInt(value *int, envViper *viper.Viper, configFileViper *viper.Viper, sectionName string, flagName string) {
	var confRet = configFileViper.GetInt(getConfName(sectionName, flagName))
	if confRet != 0 {
		*value = confRet
		return
	}

	var envRet = envViper.GetInt(getEnvName(sectionName, flagName))
	if envRet != 0 {
		*value = envRet
		return
	}
}

// ResetConfBool resets Bool value to value from config files and system environment variable
func ResetConfBool(value *bool, envViper *viper.Viper, configFileViper *viper.Viper, sectionName string, flagName string) {
	var confRet = configFileViper.GetBool(getConfName(sectionName, flagName))
	if confRet {
		*value = confRet
		return
	}

	var envRet = envViper.GetBool(getEnvName(sectionName, flagName))
	if envRet {
		*value = envRet
		return
	}
}

// ResetConfString resets String value to value from config files and system environment variable
func ResetConfString(value *string, envViper *viper.Viper, configFileViper *viper.Viper, sectionName string, flagName string) {
	var confRet = configFileViper.GetString(getConfName(sectionName, flagName))
	if confRet != "" {
		*value = confRet
		return
	}

	var envRet = envViper.GetString(getEnvName(sectionName, flagName))
	if envRet != "" {
		*value = envRet
		return
	}
}
