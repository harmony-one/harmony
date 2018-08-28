package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/dedis/kyber"
	"github.com/simple-rules/harmony-benchmark/crypto"
	"github.com/simple-rules/harmony-benchmark/crypto/pki"
	"io"
	"io/ioutil"
	"os"
)

func main() {
	// Account subcommands
	//accountNewCommand := flag.NewFlagSet("new", flag.ExitOnError)
	//accountListCommand := flag.NewFlagSet("list", flag.ExitOnError)
	//
	//// Transaction subcommands
	//transactionNewCommand := flag.NewFlagSet("new", flag.ExitOnError)
	//
	//// Account subcommand flag pointers
	//// Adding a new choice for --metric of 'substring' and a new --substring flag
	//accountNewPtr := accountNewCommand.Bool("new", false, "N/A")
	//accountListPtr := accountNewCommand.Bool("new", false, "N/A")
	//
	//// Transaction subcommand flag pointers
	//transactionNewPtr := transactionNewCommand.String("text", "", "Text to parse. (Required)")

	// Verify that a subcommand has been provided
	// os.Arg[0] is the main command
	// os.Arg[1] will be the subcommand
	if len(os.Args) < 2 {
		fmt.Println("account or transaction subcommand is required")
		os.Exit(1)
	}

	// Switch on the subcommand
	// Parse the flags for appropriate FlagSet
	// FlagSet.Parse() requires a set of arguments to parse as input
	// os.Args[2:] will be all arguments starting after the subcommand at os.Args[1]
	switch os.Args[1] {
	case "account":
		switch os.Args[2] {
		case "new":
			fmt.Println("Creating new account...")

			randomBytes := [32]byte{}
			_, err := io.ReadFull(rand.Reader, randomBytes[:])

			if err != nil {
				fmt.Println("Failed to create a new private key...")
				return
			}
			priKey := crypto.Ed25519Curve.Scalar().SetBytes(randomBytes[:])
			priKeyBytes, err := priKey.MarshalBinary()
			if err != nil {
				panic("Failed to generate private key")
			}
			pubKey := pki.GetPublicKeyFromScalar(priKey)
			address := pki.GetAddressFromPublicKey(pubKey)
			StorePrivateKey(priKeyBytes)
			fmt.Printf("New account created:\nAddress: {%x}\n", address)
		case "list":
			fmt.Println("Listing existing accounts...")
			keys := ReadPrivateKeys()
			for i, key := range keys {
				address := pki.GetAddressFromPrivateKey(key)
				fmt.Printf("Account %d:\n {%x}\n", i+1, address)
			}
		case "clearAll":
			fmt.Println("Deleting existing accounts...")
			DeletePrivateKey()
		}
	case "transaction":
		switch os.Args[2] {
		case "new":
			fmt.Println("Creating new transaction...")
		}
	default:
		flag.PrintDefaults()
		os.Exit(1)
	}
}

func StorePrivateKey(priKey []byte) {
	f, err := os.OpenFile("keystore", os.O_APPEND|os.O_WRONLY, 0644)

	if err != nil {
		panic("Failed to open keystore")
	}
	_, err = f.Write(priKey)

	if err != nil {
		panic("Failed to write to keystore")
	}
	f.Close()
}

func DeletePrivateKey() {
	ioutil.WriteFile("keystore", []byte{}, 0644)
}

func ReadPrivateKeys() []kyber.Scalar {
	keys, err := ioutil.ReadFile("keystore")
	if err != nil {
		return []kyber.Scalar{}
	}
	keyScalars := []kyber.Scalar{}
	for i := 0; i < len(keys); i += 32 {
		priKey := crypto.Ed25519Curve.Scalar()
		priKey.UnmarshalBinary(keys[i : i+32])
		keyScalars = append(keyScalars, priKey)
	}
	return keyScalars
}
