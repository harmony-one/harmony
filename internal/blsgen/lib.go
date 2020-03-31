package blsgen

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"
	"errors"

	ffi_bls "github.com/harmony-one/bls/ffi/go/bls"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/harmony-one/harmony/crypto/bls"
)

type AwsConfiguration struct {
	AccessKey string `json:"aws_access_key_id"`
	SecretKey string `json:"aws_secret_access_key"`
	Region    string `json:"aws_region"`
}

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

// GenBlsKeyWithPassPhrase generates bls key with passphrase and write into disk.
func GenBlsKeyWithPassPhrase(passphrase string) (*ffi_bls.SecretKey, string, error) {
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

// WritePriKeyWithPassPhrase writes encrypted key with passphrase.
func WritePriKeyWithPassPhrase(privateKey *ffi_bls.SecretKey, passphrase string) (string, error) {
	publickKey := privateKey.GetPublicKey()
	fileName := publickKey.SerializeToHexStr() + ".key"
	privateKeyHex := privateKey.SerializeToHexStr()
	// Encrypt with passphrase
	encryptedPrivateKeyStr, err := encrypt([]byte(privateKeyHex), passphrase)
	if err != nil {
		return "", err
	}
	// Write to file.
	err = WriteToFile(fileName, encryptedPrivateKeyStr)
	return fileName, err
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

// LoadBlsKeyWithPassPhrase loads bls key with passphrase.
func LoadBlsKeyWithPassPhrase(fileName, passphrase string) (*ffi_bls.SecretKey, error) {
	encryptedPrivateKeyBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	for len(passphrase) > 0 && passphrase[len(passphrase)-1] == '\n' {
		passphrase = passphrase[:len(passphrase)-1]
	}
	decryptedBytes, err := decrypt(encryptedPrivateKeyBytes, passphrase)
	if err != nil {
		return nil, err
	}

	priKey := &ffi_bls.SecretKey{}
	priKey.DeserializeHexStr(string(decryptedBytes))
	return priKey, nil
}

// Readln reads aws configuratoin from prompt with a timeout
func Readln(timeout time.Duration) (string, error) {
	s := make(chan string)
	e := make(chan error)

	go func() {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadString('\n')
		if err != nil {
			e <- err
		} else {
			s <- line
		}
		close(s)
		close(e)
	}()

	select {
	case line := <-s:
		return line, nil
	case err := <-e:
		return "", err
	case <-time.After(timeout):
		return "", errors.New("Timeout")
	}
}

// LoadBlsKeyWithPassPhrase loads bls key with passphrase.
func LoadAwsCMKEncryptedBlsKey(fileName, awsSettingString string) (*ffi_bls.SecretKey, error) {
	if (awsSettingString == "") {
		return nil, errors.New("aws credential is not set")
	}

	var awsConfig AwsConfiguration
	err := json.Unmarshal([]byte(awsSettingString), &awsConfig)
	if err != nil {
		return nil, errors.New(awsSettingString +  " is not a valid JSON string for setting aws configuration.")
	}

	// Initialize a session that the aws SDK uses to load
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	// Create KMS service client
	svc := kms.New(sess, &aws.Config{
		//Region: aws.String("us-east-1"),
		Region: aws.String(awsConfig.Region),
		Credentials: credentials.NewStaticCredentials(awsConfig.AccessKey, awsConfig.SecretKey, ""),
	})

	encryptedPrivateKeyBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	unhexed := make([]byte, hex.DecodedLen(len(encryptedPrivateKeyBytes)))
	_, err = hex.Decode(unhexed, encryptedPrivateKeyBytes)
	if err != nil {
		return nil, err
	}

	clearKey, err := svc.Decrypt(&kms.DecryptInput{
		CiphertextBlob: unhexed,
	})

	if err != nil {
		return nil, err
	}

	priKey := &ffi_bls.SecretKey{}
	priKey.DeserializeHexStr(hex.EncodeToString(clearKey.Plaintext))

	fmt.Println(hex.EncodeToString(priKey.GetPublicKey().Serialize()))

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
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	return plaintext, err
}

func decryptNonHumanReadable(data []byte, passphrase string) ([]byte, error) {
	var err error
	key := []byte(createHash(passphrase))
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	nonceSize := gcm.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// LoadNonHumanReadableBlsKeyWithPassPhrase loads bls key with passphrase.
func LoadNonHumanReadableBlsKeyWithPassPhrase(fileName, passFile string) (*ffi_bls.SecretKey, error) {
	encryptedPrivateKeyBytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadFile(passFile)
	if err != nil {
		return nil, err
	}
	passphrase := string(data)
	for len(passphrase) > 0 && passphrase[len(passphrase)-1] == '\n' {
		passphrase = passphrase[:len(passphrase)-1]
	}
	decryptedBytes, err := decryptNonHumanReadable(encryptedPrivateKeyBytes, passphrase)
	if err != nil {
		return nil, err
	}

	priKey := &ffi_bls.SecretKey{}
	priKey.DeserializeHexStr(string(decryptedBytes))
	return priKey, nil
}
