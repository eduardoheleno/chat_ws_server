package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
)

func GenerateNodeHash() string {
	randomBytes := make([]byte, 32)

	_, err := rand.Read(randomBytes)
	if err != nil {
		log.Panicln("Failed to generate random bytes")
	}
	hasher := sha256.New()
	hasher.Write(randomBytes)
	hashBytes := hasher.Sum(nil)

	hashString := hex.EncodeToString(hashBytes)

	return hashString
}
