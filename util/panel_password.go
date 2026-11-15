package util

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
)

// All supported encoded hashes use these fixed bounds.
const panelPasswordPrefix = "$argon2id$v=19$m=19456,t=2,p=1$"

func panelPasswordKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 2, 19*1024, 1, 32)
}

func HashPanelPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", errors.New("generate panel password salt")
	}
	enc := base64.RawStdEncoding
	return panelPasswordPrefix + enc.EncodeToString(salt) + "$" + enc.EncodeToString(panelPasswordKey(password, salt)), nil
}

func VerifyPanelPassword(encoded, password string) bool {
	value, ok := strings.CutPrefix(encoded, panelPasswordPrefix)
	saltText, hashText, valid := strings.Cut(value, "$")
	enc := base64.RawStdEncoding
	if !ok || !valid || len(saltText) != enc.EncodedLen(16) || len(hashText) != enc.EncodedLen(32) {
		return false
	}
	salt, saltErr := enc.DecodeString(saltText)
	hash, hashErr := enc.DecodeString(hashText)
	return saltErr == nil && hashErr == nil && subtle.ConstantTimeCompare(hash, panelPasswordKey(password, salt)) == 1
}
