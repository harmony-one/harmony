package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/iancoleman/strcase"
)

// EmbedFile Text file embed script for go:generate.
// This script embeds a text file located at filePath into a string constant
// named as constName defined in a golang source file located at the current
// go:generate path.
func EmbedFile(filePath string, constName string) {
	// validate inputs
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		panic(fmt.Sprintf("File %s does not exist", filePath))
	}

	if strcase.ToLowerCamel(constName) != constName {
		panic(fmt.Sprintf("constName %s is not in lower camel-case", constName))
	}

	// generate go file
	var err error
	fileName := filepath.Base(filePath)
	generatedFileName := "generated_" + strcase.ToSnake(fileName) + ".go"
	out, err := os.Create(generatedFileName)
	if err != nil {
		panic(err)
	}

	_, err = out.Write([]byte(fmt.Sprintf("package %s\n\nconst (\n", os.Getenv("GOPACKAGE"))))
	if err != nil {
		panic(err)
	}

	_, err = out.Write([]byte("\t" + constName + " = `"))
	if err != nil {
		panic(err)
	}

	f, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(out, f)
	if err != nil {
		panic(err)
	}

	_, err = out.Write([]byte("`\n"))
	if err != nil {
		panic(err)
	}

	_, err = out.Write([]byte(")\n"))
	if err != nil {
		panic(err)
	}
}
