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
	return verifyPanelPassword(encoded, password, panelPasswordKey)
}

func verifyPanelPassword(encoded, password string, derive func(string, []byte) []byte) bool {
	value, hasPrefix := strings.CutPrefix(encoded, panelPasswordPrefix)
	saltText, hashText, hasSeparator := strings.Cut(value, "$")
	enc := base64.RawStdEncoding

	// Use fixed-size dummy buffers when the administrator has no usable hash.
	// Even a match against dummy data must never authenticate.
	salt := make([]byte, 16)
	hash := make([]byte, 32)
	valid := false
	if hasPrefix && hasSeparator && len(saltText) == enc.EncodedLen(len(salt)) && len(hashText) == enc.EncodedLen(len(hash)) {
		_, saltErr := enc.Decode(salt, []byte(saltText))
		_, hashErr := enc.Decode(hash, []byte(hashText))
		valid = saltErr == nil && hashErr == nil
	}
	matches := subtle.ConstantTimeCompare(hash, derive(password, salt)) == 1
	return valid && matches
}
