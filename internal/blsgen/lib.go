package blsgen

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/service/kms"
	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"
	"github.com/harmony-one/harmony/crypto/bls"
	"github.com/pkg/errors"
)

// GenBLSKeyWithPassPhrase generates bls key with passphrase and write into disk.
func GenBLSKeyWithPassPhrase(passphrase string) (*ffi_bls.SecretKey, string, error) {
	privateKey := bls.RandPrivateKey()
	publickKey := privateKey.GetPublicKey()
	fileName := publickKey.SerializeToHexStr() + ".key"
	privateKeyHex := privateKey.SerializeToHexStr()
	// Encrypt with passphrase
	encryptedPrivateKeyStr, err := encrypt([]byte(privateKeyHex), passphrase)
	if err != nil {
		return nil, "", err
	}
	// Write to file.
	err = WriteToFile(fileName, encryptedPrivateKeyStr)
	return privateKey, fileName, err
}

// WriteToFile will print any string of text to a file safely by
// checking for errors and syncing at the end.
func WriteToFile(filename string, data string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.WriteString(file, data)
	if err != nil {
		return err
	}
	return file.Sync()
}

// LoadBLSKeyWithPassPhrase loads bls key with passphrase.
func LoadBLSKeyWithPassPhrase(fileName, passphrase string) (*ffi_bls.SecretKey, error) {
	encryptedPrivateKeyBytes, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "attempted to load from %s", fileName)
	}
	passphrase = strings.TrimSpace(passphrase)

	decryptedBytes, err := decrypt(encryptedPrivateKeyBytes, passphrase)
	if err != nil {
		return nil, err
	}

	priKey := &ffi_bls.SecretKey{}
	if err := priKey.DeserializeHexStr(string(decryptedBytes)); err != nil {
		return nil, errors.Wrapf(
			err, "could not deserialize byte content of %s as BLS secret key", fileName,
		)
	}
	return priKey, nil
}

// LoadAwsCMKEncryptedBLSKey loads aws encrypted bls key.
func LoadAwsCMKEncryptedBLSKey(fileName string, kmsClient *kms.KMS) (*ffi_bls.SecretKey, error) {
	encryptedPrivateKeyBytes, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errors.Wrapf(err, "fail read at: %s", fileName)
	}

	unhexed := make([]byte, hex.DecodedLen(len(encryptedPrivateKeyBytes)))
	if _, err = hex.Decode(unhexed, encryptedPrivateKeyBytes); err != nil {
		return nil, err
	}

	clearKey, err := kmsClient.Decrypt(&kms.DecryptInput{
		CiphertextBlob: unhexed,
	})

	if err != nil {
		return nil, err
	}

	priKey := &ffi_bls.SecretKey{}
	if err = priKey.DeserializeHexStr(hex.EncodeToString(clearKey.Plaintext)); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize the decrypted bls private key")
	}

	return priKey, nil
}

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}

func encrypt(data []byte, passphrase string) (string, error) {
	block, _ := aes.NewCipher([]byte(createHash(passphrase)))
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return hex.EncodeToString(ciphertext), nil
}

func decrypt(encrypted []byte, passphrase string) (decrypted []byte, err error) {
	unhexed := make([]byte, hex.DecodedLen(len(encrypted)))
	if _, err = hex.Decode(unhexed, encrypted); err == nil {
		if decrypted, err = decryptRaw(unhexed, passphrase); err == nil {
			return decrypted, nil
		}
	}
	// At this point err != nil, either from hex decode or from decryptRaw.
	decrypted, binErr := decryptRaw(encrypted, passphrase)
	if binErr != nil {
		// Disregard binary decryption error and return the original error,
		// because our canonical form is hex and not binary.
		return nil, err
	}
	return decrypted, nil
}

func decryptRaw(data []byte, passphrase string) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("unable to decrypt raw data with the provided passphrase; the data is empty")
	}
	var err error
	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("failed to decrypt raw data with the provided passphrase; the data size is invalid")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	return plaintext, err
}
