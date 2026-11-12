package identity

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"golang.org/x/crypto/argon2"
	"strings"
	"unicode/utf8"
)

const passwordPrefix = "$argon2id$v=19$m=32768,t=2,p=1$"

func HashPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < 12 || utf8.RuneCountInString(password) > 128 {
		return "", errors.New("password must contain 12 to 128 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 2, 32768, 1, 32)
	return passwordPrefix + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(hash), nil
}
func VerifyPassword(password, encoded string) bool {
	if len(password) > 512 || !strings.HasPrefix(encoded, passwordPrefix) {
		return false
	}
	fields := strings.Split(strings.TrimPrefix(encoded, passwordPrefix), "$")
	if len(fields) != 2 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(fields[0])
	if err != nil || len(salt) != 16 {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(fields[1])
	if err != nil || len(expected) != 32 {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, 2, 32768, 1, 32)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
func dummyPasswordWork(password string) {
	argon2.IDKey([]byte(password), []byte("unknown-user-key"), 2, 32768, 1, 32)
}
