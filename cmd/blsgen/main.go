package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
)

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

func encrypt(data []byte, passphrase string) []byte {
	block, _ := aes.NewCipher([]byte(createHash(passphrase)))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		panic(err.Error())
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext
}

func blsKeyGen(passphrase string) (string, string, []byte) {
	privateKey := bls.RandPrivateKey()
	publickKey := privateKey.GetPublicKey()
	fmt.Printf("{Private: \"%s\", Public: \"%s\"},\n", privateKey.GetHexString(), publickKey.GetHexString())
	privateKeyHex := privateKey.GetHexString()

	// encryptedPrivateKeyBytes := encrypt([]byte(privateKeyHex), passphrase)
	return privateKey.GetHexString(), publickKey.GetHexString(), encrypt([]byte(privateKeyHex), passphrase)
	// encryptedPrivateKeyStr := hex.EncodeToString(encryptedPrivateKeyBytes)
}

func blsKeyGenAndWriteToFile() {}

func decrypt(data []byte, passphrase string) []byte {
	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		panic(err.Error())
	}
	return plaintext
}

func encryptFile(filename string, data []byte, passphrase string) {
	f, _ := os.Create(filename)
	defer f.Close()
	f.Write(encrypt(data, passphrase))
}

func decryptFile(filename string, passphrase string) []byte {
	data, _ := ioutil.ReadFile(filename)
	return decrypt(data, passphrase)
}

func main() {
	privateKey := bls.RandPrivateKey()
	publickKey := privateKey.GetPublicKey()
	fmt.Printf("{Private: \"%s\", Public: \"%s\"},\n", privateKey.GetHexString(), publickKey.GetHexString())
	privateKeyHex := privateKey.GetHexString()

	encryptedPrivateKeyBytes := encrypt([]byte(privateKeyHex), "minh")
	encryptedPrivateKeyStr := hex.EncodeToString(encryptedPrivateKeyBytes)

	// the WriteFile method returns an error if unsuccessful
	err := ioutil.WriteFile("myfile.data", encryptedPrivateKeyBytes, 0777)
	// handle this error
	if err != nil {
		// print it out
		fmt.Println(err)
	}

	fmt.Println(encryptedPrivateKeyStr)

	encryptedPrivateKeyBytes, err = ioutil.ReadFile("myfile.data")
	if err != nil {
		fmt.Println(err)
	}

	decryptedBytes := decrypt(encryptedPrivateKeyBytes, "minh")

	priKey := &ffi_bls.SecretKey{}
	priKey.SetHexString(string(decryptedBytes))
	fmt.Printf("{Private: \"%s\"},\n", priKey.GetHexString())
}
