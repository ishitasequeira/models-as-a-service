package api_keys

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

const (
	// KeyPrefix is the prefix for all OpenShift AI API keys
	// Per Feature Refinement: "Simple Opaque Key Format" - keys must be short, opaque strings
	// with a recognizable prefix matching industry standards (OpenAI, Stripe, GitHub).
	KeyPrefix = "sk-oai-"

	// entropyBytes is the number of random bytes to generate (256 bits).
	entropyBytes = 32

	// displayPrefixLength is the number of chars to show in the display prefix (after sk-oai-).
	displayPrefixLength = 12

	// saltBytes is the number of random bytes to generate for salt (256 bits).
	saltBytes = 32
)

// GenerateAPIKey creates a new API key with format: sk-oai-{base62_encoded_256bit_random}
// Returns: (plaintext_key, sha256_hash, display_prefix, error)
//
// Security properties (per Feature Refinement "Key Format & Security"):
// - 256 bits of cryptographic entropy
// - Base62 encoding (alphanumeric only, URL-safe)
// - SHA-256 hash for storage (plaintext never stored)
// - Display prefix for UI identification.
//
//nolint:nonamedreturns // Named returns improve readability for multiple return values.
func GenerateAPIKey() (plaintext, hash, prefix string, err error) {
	// 1. Generate 32 bytes (256 bits) of cryptographic entropy
	entropy := make([]byte, entropyBytes)
	if _, err := rand.Read(entropy); err != nil {
		return "", "", "", fmt.Errorf("failed to generate entropy: %w", err)
	}

	// 2. Encode to base62 (alphanumeric only, no special characters)
	encoded := encodeBase62(entropy)

	// 3. Construct key with OpenShift AI prefix
	plaintext = KeyPrefix + encoded

	// 4. Compute salted SHA-256 hash for storage
	hash, err = HashAPIKeyWithSalt(plaintext)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to hash API key: %w", err)
	}

	// 5. Create display prefix (first 12 chars + ellipsis)
	if len(encoded) >= displayPrefixLength {
		prefix = KeyPrefix + encoded[:displayPrefixLength] + "..."
	} else {
		prefix = KeyPrefix + encoded + "..."
	}

	return plaintext, hash, prefix, nil
}

// HashAPIKeyWithSalt computes salted SHA-256 hash of an API key.
// Returns format: <salt_hex>:<hash_hex> (both 64 hex chars).
// Uses 256-bit random salt for rainbow table protection.
func HashAPIKeyWithSalt(key string) (string, error) {
	// Generate cryptographically secure random salt
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Compute salted hash: SHA-256(key + salt)
	h := sha256.New()
	h.Write([]byte(key))
	h.Write(salt)
	hash := h.Sum(nil)

	// Return format: <salt_hex>:<hash_hex>
	return fmt.Sprintf("%s:%s",
		hex.EncodeToString(salt),
		hex.EncodeToString(hash)), nil
}

// ValidateAPIKeyHash validates an API key against its stored hash.
// Expected format: <salt_hex>:<hash_hex> (both 64 hex chars).
func ValidateAPIKeyHash(key, storedHash string) bool {
	// Parse <salt_hex>:<hash_hex>
	parts := strings.Split(storedHash, ":")
	if len(parts) != 2 {
		return false
	}

	saltHex, hashHex := parts[0], parts[1]

	// Validate hex string lengths (32 bytes = 64 hex chars each)
	if len(saltHex) != 64 || len(hashHex) != 64 {
		return false
	}

	// Decode salt from hex
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}

	// Decode expected hash from hex
	expectedHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return false
	}

	// Compute hash of key + salt
	h := sha256.New()
	h.Write([]byte(key))
	h.Write(salt)
	computedHash := h.Sum(nil)

	// Compare hashes using constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(expectedHash, computedHash) == 1
}

// IsValidKeyFormat checks if a key has the correct sk-oai-* prefix and valid base62 body.
func IsValidKeyFormat(key string) bool {
	if !strings.HasPrefix(key, KeyPrefix) {
		return false
	}

	body := key[len(KeyPrefix):]
	if len(body) == 0 {
		return false // Reject empty body
	}

	// Validate base62 charset (0-9, A-Z, a-z)
	for _, c := range body {
		if (c < '0' || c > '9') && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}

	return true
}

// encodeBase62 converts byte array to base62 string
// Base62 uses 0-9, A-Z, a-z (no special characters, URL-safe).
func encodeBase62(data []byte) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	n := new(big.Int).SetBytes(data)
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result []byte
	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		result = append([]byte{alphabet[mod.Int64()]}, result...)
	}

	// Handle zero input
	if len(result) == 0 {
		return string(alphabet[0])
	}

	return string(result)
}
