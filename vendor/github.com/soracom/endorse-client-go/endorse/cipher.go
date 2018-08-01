package endorse

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"io"

	"github.com/pkg/errors"
)

func generateRandomIV() []byte {
	iv := make([]byte, aes.BlockSize)
	_, err := io.ReadFull(rand.Reader, iv)
	if err != nil {
		return nil
	}
	return iv
}

func encode(plainData []byte, password []byte, iv []byte) ([]byte, error) {
	key := makeKeyFromPassword(password)

	bc, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// IV needs to be unique, but doesn't have to be secure.
	// It's common to put it at the beginning of the encoded data.
	encodedData := make([]byte, aes.BlockSize+len(plainData))
	copy(encodedData[:aes.BlockSize], iv)

	stream := cipher.NewCFBEncrypter(bc, iv)
	stream.XORKeyStream(encodedData[aes.BlockSize:], plainData)

	return encodedData, nil
}

func decode(encodedData []byte, password []byte) ([]byte, error) {
	key := makeKeyFromPassword(password)

	bc, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(encodedData) < aes.BlockSize {
		return nil, errors.New("encoded data block size is too short")
	}

	// IV needs to be unique, but doesn't have to be secure.
	// It's common to put it at the beginning of the encoded data.
	iv := encodedData[:aes.BlockSize]
	encodedData = encodedData[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(bc, iv)
	plainData := make([]byte, len(encodedData))
	stream.XORKeyStream(plainData, encodedData)

	return plainData, nil
}

func calculateSignature(data []byte, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func makeKeyFromPassword(password []byte) []byte {
	bs := aes.BlockSize
	padLen := bs - (len(password) % bs)
	pad := bytes.Repeat([]byte{0x00}, padLen)
	paddedPassword := append(password, pad...)
	return paddedPassword[:bs]
}
