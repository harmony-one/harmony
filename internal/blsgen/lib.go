package blsgen

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
)

// Constants for bls lib.
const (
	BlsPriKeyStore = ".hmy/blsstore/"
)

func toISO8601(t time.Time) string {
	var tz string
	name, offset := t.Zone()
	if name == "UTC" {
		tz = "Z"
	} else {
		tz = fmt.Sprintf("%03d00", offset/3600)
	}
	return fmt.Sprintf("%04d-%02d-%02dT%02d-%02d-%02d.%09d%s",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), tz)
}

// WritePrivateKeyWithPassPhrase encrypt the key with passphrase and write into disk.
func WritePrivateKeyWithPassPhrase(privateKey *ffi_bls.SecretKey, passphrase string) string {
	publickKey := privateKey.GetPublicKey()
	fileName := publickKey.SerializeToHexStr() + ".key"
	privateKeyHex := privateKey.SerializeToHexStr()
	// Encrypt with passphrase
	encryptedPrivateKeyStr := encrypt([]byte(privateKeyHex), passphrase)

	// Write to file.
	err := WriteToFile(fileName, encryptedPrivateKeyStr)
	check(err)
	return fileName
}

func visit(files *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		fi, err := os.Stat(path)
		if !fi.Mode().IsDir() {
			*files = append(*files, path)
		}
		return nil
	}
}

// FindBlsPriKey finds bls private key from bls private store with passphrase.
func FindBlsPriKey(BlsPublicKey, blsPass string, consensusPriKey *ffi_bls.SecretKey) bool {
	var files []string
	var err error
	if _, err = os.Stat(BlsPriKeyStore); os.IsNotExist(err) {
		return false
	}

	err = filepath.Walk(BlsPriKeyStore, visit(&files))
	check(err)
	for _, f := range files {
		k := LoadBlsKeyWithPassPhrase(f, blsPass)
		if k.GetPublicKey().SerializeToHexStr() == BlsPublicKey {
			consensusPriKey = k
			return true
		}
	}
	return false
}

// GenBlsKeyWithPassPhrase generates bls key with passphrase and write into disk.
func GenBlsKeyWithPassPhrase(passphrase string) (*ffi_bls.SecretKey, string) {
	privateKey := bls.RandPrivateKey()
	publickKey := privateKey.GetPublicKey()
	fileName := publickKey.SerializeToHexStr() + ".key"
	privateKeyHex := privateKey.SerializeToHexStr()
	// Encrypt with passphrase
	encryptedPrivateKeyStr := encrypt([]byte(privateKeyHex), passphrase)
	// Write to file.
	err := WriteToFile(fileName, encryptedPrivateKeyStr)
	check(err)
	return privateKey, fileName
}

// WriteToFile will print any string of text to a file safely by
// checking for errors and syncing at the end.
func WriteToFile(filename string, data string) error {
	file, err := os.Create(filename)
	check(err)
	defer file.Close()

	_, err = io.WriteString(file, data)
	check(err)
	return file.Sync()
}

// LoadBlsKeyWithPassPhrase loads bls key with passphrase.
func LoadBlsKeyWithPassPhrase(fileName, passphrase string) *ffi_bls.SecretKey {
	encryptedPrivateKeyBytes, err := ioutil.ReadFile(fileName)
	check(err)
	encryptedPrivateKeyStr := string(encryptedPrivateKeyBytes)
	decryptedBytes := decrypt(encryptedPrivateKeyStr, passphrase)

	priKey := &ffi_bls.SecretKey{}
	priKey.DeserializeHexStr(string(decryptedBytes))
	return priKey
}

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

func encrypt(data []byte, passphrase string) string {
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
	return hex.EncodeToString(ciphertext)
}

func decrypt(encryptedStr string, passphrase string) []byte {
	var err error
	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err.Error())
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}
	data, err := hex.DecodeString(encryptedStr)
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

func check(err error) {
	if err != nil {
		panic(err)
	}
}
