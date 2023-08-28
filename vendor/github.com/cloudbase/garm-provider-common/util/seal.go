package util

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"

	"github.com/minio/sio"
	"github.com/pkg/errors"
	"golang.org/x/crypto/hkdf"
)

type Envelope struct {
	Nonce [32]byte `json:"nonce"`
	Data  []byte   `json:"data"`
}

// Seal will encrypt the given data using a derived key from the given passphrase.
// This function is meant to be used with small datasets like passwords, keys and
// secrets of any type, before they are saved to disk.
func Seal(data []byte, passphrase []byte) ([]byte, error) {
	if len(passphrase) != 32 {
		return nil, fmt.Errorf("invalid passphrase length (expected length 32 characters)")
	}

	var nonce [32]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to read random data: %w", err)
	}

	// derive an encryption key from the master key and the nonce
	var key [32]byte
	kdf := hkdf.New(sha256.New, passphrase, nonce[:], nil)
	if _, err := io.ReadFull(kdf, key[:]); err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	input := bytes.NewReader(data)
	output := bytes.NewBuffer(nil)

	if _, err := sio.Encrypt(output, input, sio.Config{Key: key[:]}); err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}
	envelope := Envelope{
		Data:  output.Bytes(),
		Nonce: nonce,
	}
	asJs, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal envelope: %w", err)
	}
	return asJs, nil
}

// Unseal will decrypt the given data using a derived key from the given passphrase.
// This function is meant to be used with small datasets like passwords, keys and
// secrets of any type, after they are read from disk.
func Unseal(data []byte, passphrase []byte) ([]byte, error) {
	if len(passphrase) != 32 {
		return nil, fmt.Errorf("invalid passphrase length (expected length 32 characters)")
	}

	var envelope Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Aes256Decode(data, string(passphrase))
	}

	// derive an encryption key from the master key and the nonce
	var key [32]byte
	kdf := hkdf.New(sha256.New, passphrase, envelope.Nonce[:], nil)
	if _, err := io.ReadFull(kdf, key[:]); err != nil {
		return nil, fmt.Errorf("failed to derive encryption key: %w", err)
	}

	input := bytes.NewReader(envelope.Data)
	output := bytes.NewBuffer(nil)

	if _, err := sio.Decrypt(output, input, sio.Config{Key: key[:]}); err != nil {
		return nil, fmt.Errorf("failed to decrypt data: %w", err)
	}

	return output.Bytes(), nil
}

func Aes256Encode(target []byte, passphrase string) ([]byte, error) {
	if len(passphrase) != 32 {
		return nil, fmt.Errorf("invalid passphrase length (expected length 32 characters)")
	}

	block, err := aes.NewCipher([]byte(passphrase))
	if err != nil {
		return nil, errors.Wrap(err, "creating cipher")
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating new aead")
	}

	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, errors.Wrap(err, "creating nonce")
	}

	ciphertext := aesgcm.Seal(nonce, nonce, target, nil)
	return ciphertext, nil
}

func Aes256EncodeString(target string, passphrase string) ([]byte, error) {
	if len(passphrase) != 32 {
		return nil, fmt.Errorf("invalid passphrase length (expected length 32 characters)")
	}

	return Aes256Encode([]byte(target), passphrase)
}

func Aes256Decode(target []byte, passphrase string) ([]byte, error) {
	if len(passphrase) != 32 {
		return nil, fmt.Errorf("invalid passphrase length (expected length 32 characters)")
	}

	block, err := aes.NewCipher([]byte(passphrase))
	if err != nil {
		return nil, errors.Wrap(err, "creating cipher")
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.Wrap(err, "creating new aead")
	}

	nonceSize := aesgcm.NonceSize()
	if len(target) < nonceSize {
		return nil, fmt.Errorf("failed to decrypt text")
	}

	nonce, ciphertext := target[:nonceSize], target[nonceSize:]
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt text")
	}
	return plaintext, nil
}

func Aes256DecodeString(target []byte, passphrase string) (string, error) {
	data, err := Aes256Decode(target, passphrase)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
