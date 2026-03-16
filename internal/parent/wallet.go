package parent

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

// GenerateCustodialWallet creates a new Ethereum key pair for a parent.
// Returns: address (hex string with 0x prefix), private key (hex string without 0x), error.
func GenerateCustodialWallet() (address string, privKeyHex string, err error) {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}

	addr := crypto.PubkeyToAddress(privateKey.PublicKey)
	privBytes := crypto.FromECDSA(privateKey)
	privHex := hex.EncodeToString(privBytes)

	return addr.Hex(), privHex, nil
}

// EncryptPrivateKey encrypts a hex-encoded private key using AES-256-GCM.
// The encryptionKey must be exactly 32 bytes.
func EncryptPrivateKey(privKeyHex string, encryptionKey []byte) (string, error) {
	plaintext := []byte(privKeyHex)

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(ciphertext), nil
}

// DecryptPrivateKey decrypts an AES-256-GCM encrypted private key.
func DecryptPrivateKey(encryptedHex string, encryptionKey []byte) (string, error) {
	ciphertext, err := hex.DecodeString(encryptedHex)
	if err != nil {
		return "", fmt.Errorf("decode hex: %w", err)
	}

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// DeriveEncryptionKey derives a 32-byte AES key from a master secret.
// For the hackathon: use a server-wide master key (env variable or flag).
// For production: derive per-parent from their password via scrypt/PBKDF2.
func DeriveEncryptionKey(masterSecret string) []byte {
	hash := sha256.Sum256([]byte(masterSecret))
	return hash[:]
}

// PrivKeyHexToECDSA converts a hex-encoded private key back to an *ecdsa.PrivateKey
// for signing Ethereum transactions.
func PrivKeyHexToECDSA(privKeyHex string) (*ecdsa.PrivateKey, error) {
	return crypto.HexToECDSA(privKeyHex)
}
