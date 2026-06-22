package id

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

func NewPublicID(prefix string) string {
	return prefix + "_" + randomString(16)
}

func NewToken(prefix string) string {
	return prefix + "_" + randomString(32)
}

func randomString(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	return strings.ToLower(encoded)
}
