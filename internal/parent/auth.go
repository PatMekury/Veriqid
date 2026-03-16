package parent

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/mail"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost = 12
	otpLength  = 6
)

// ValidateEmail checks if the email format is valid (RFC 5322).
func ValidateEmail(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

// ValidatePhone checks if the phone number is in a reasonable format.
// Accepts digits-only, 10-15 characters (with optional leading +).
func ValidatePhone(phone string) error {
	cleaned := regexp.MustCompile(`[^0-9+]`).ReplaceAllString(phone, "")
	digitsOnly := regexp.MustCompile(`[^0-9]`).ReplaceAllString(cleaned, "")
	if len(digitsOnly) < 10 || len(digitsOnly) > 15 {
		return fmt.Errorf("phone number must be 10-15 digits")
	}
	return nil
}

// ValidatePassword checks password strength requirements.
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	if len(password) > 128 {
		return fmt.Errorf("password must be at most 128 characters")
	}
	return nil
}

// HashPassword creates a bcrypt hash from a plaintext password.
func HashPassword(password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateOTP creates a cryptographically random 6-digit code.
func GenerateOTP() (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(otpLength)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generate OTP: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// NormalizeEmail lowercases and trims whitespace from an email.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// NormalizePhone strips non-digit characters except leading +.
func NormalizePhone(phone string) string {
	cleaned := strings.TrimSpace(phone)
	if strings.HasPrefix(cleaned, "+") {
		return "+" + regexp.MustCompile(`[^0-9]`).ReplaceAllString(cleaned[1:], "")
	}
	return regexp.MustCompile(`[^0-9]`).ReplaceAllString(cleaned, "")
}
