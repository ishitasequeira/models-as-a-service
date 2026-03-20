package api_keys_test

import (
	"regexp"
	"testing"

	"github.com/opendatahub-io/models-as-a-service/maas-api/internal/api_keys"
)

const testAPIKey = "sk-oai-test123"

func TestGenerateAPIKey(t *testing.T) {
	plaintext, hash, prefix, err := api_keys.GenerateAPIKey()

	if err != nil {
		t.Fatalf("GenerateAPIKey() returned error: %v", err)
	}

	// Test 1: Key has correct prefix
	if !api_keys.IsValidKeyFormat(plaintext) {
		t.Errorf("GenerateAPIKey() key missing prefix 'sk-oai-': got %q", plaintext)
	}

	// Test 2: Hash is format <salt_hex>:<hash_hex> (both 64 hex chars)
	hashRegex := regexp.MustCompile(`^[0-9a-f]{64}:[0-9a-f]{64}$`)
	if !hashRegex.MatchString(hash) {
		t.Errorf("GenerateAPIKey() hash not in expected format: got %q", hash)
	}

	// Test 4: Prefix has correct format
	prefixRegex := regexp.MustCompile(`^sk-oai-[A-Za-z0-9]{12}\.\.\.$`)
	if !prefixRegex.MatchString(prefix) {
		t.Errorf("GenerateAPIKey() prefix format incorrect: got %q", prefix)
	}

	// Test 5: Key is alphanumeric after prefix (base62)
	keyBody := plaintext[len(api_keys.KeyPrefix):]
	alphanumRegex := regexp.MustCompile("^[A-Za-z0-9]+$")
	if !alphanumRegex.MatchString(keyBody) {
		t.Errorf("GenerateAPIKey() key body not alphanumeric: got %q", keyBody)
	}

	// Test 6: Key body is sufficiently long (256 bits → ~43 base62 chars)
	if len(keyBody) < 40 {
		t.Errorf("GenerateAPIKey() key body too short: got %d chars, want >= 40", len(keyBody))
	}
}

func TestGenerateAPIKey_Uniqueness(t *testing.T) {
	// Generate multiple keys and ensure they're unique
	keys := make(map[string]bool)
	hashes := make(map[string]bool)

	for i := range 100 {
		plaintext, hash, _, err := api_keys.GenerateAPIKey()
		if err != nil {
			t.Fatalf("GenerateAPIKey() iteration %d returned error: %v", i, err)
		}

		if keys[plaintext] {
			t.Errorf("GenerateAPIKey() generated duplicate key on iteration %d", i)
		}
		keys[plaintext] = true

		if hashes[hash] {
			t.Errorf("GenerateAPIKey() generated duplicate hash on iteration %d", i)
		}
		hashes[hash] = true
	}
}

func TestHashAPIKeyWithSalt(t *testing.T) {
	testKey := testAPIKey
	
	// Test 1: Function returns salted hash
	hash1, err := api_keys.HashAPIKeyWithSalt(testKey)
	if err != nil {
		t.Fatalf("HashAPIKeyWithSalt() returned error: %v", err)
	}

	// Test 2: Hash has correct format <salt_hex>:<hash_hex> (both 64 hex chars)
	hashRegex := regexp.MustCompile(`^[0-9a-f]{64}:[0-9a-f]{64}$`)
	if !hashRegex.MatchString(hash1) {
		t.Errorf("HashAPIKeyWithSalt() hash not in expected format: got %q", hash1)
	}

	// Test 3: Different calls produce different hashes (due to random salt)
	hash2, err := api_keys.HashAPIKeyWithSalt(testKey)
	if err != nil {
		t.Fatalf("HashAPIKeyWithSalt() second call returned error: %v", err)
	}
	if hash1 == hash2 {
		t.Error("HashAPIKeyWithSalt() should produce different hashes due to random salt")
	}

	// Test 4: Different keys produce different hashes
	differentHash, err := api_keys.HashAPIKeyWithSalt("sk-oai-different")
	if err != nil {
		t.Fatalf("HashAPIKeyWithSalt() with different key returned error: %v", err)
	}
	if hash1 == differentHash {
		t.Error("HashAPIKeyWithSalt() should produce different hashes for different keys")
	}
}

func TestValidateAPIKeyHash(t *testing.T) {
	testKey := "sk-oai-test123"

	t.Run("ValidateSaltedHash", func(t *testing.T) {
		// Create salted hash
		saltedHash, err := api_keys.HashAPIKeyWithSalt(testKey)
		if err != nil {
			t.Fatalf("HashAPIKeyWithSalt() returned error: %v", err)
		}

		// Should validate successfully
		if !api_keys.ValidateAPIKeyHash(testKey, saltedHash) {
			t.Error("ValidateAPIKeyHash() should validate correct salted hash")
		}

		// Should fail with wrong key
		if api_keys.ValidateAPIKeyHash("sk-oai-wrong", saltedHash) {
			t.Error("ValidateAPIKeyHash() should reject wrong key against salted hash")
		}
	})

	t.Run("RejectInvalidFormats", func(t *testing.T) {
		testCases := []string{
			"invalid-hash",
			"short:hash",
			"not-hex:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef:not-hex",
			// Wrong number of parts
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"a:b:c",
			"v2:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef:hash",
			// Wrong length (not 64 hex chars)
			"abc123:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef:abc123",
		}

		for _, invalidHash := range testCases {
			if api_keys.ValidateAPIKeyHash(testKey, invalidHash) {
				t.Errorf("ValidateAPIKeyHash() should reject invalid format: %q", invalidHash)
			}
		}
	})
}

func TestConstantTimeCompare(t *testing.T) {
	// This test verifies that our constant-time comparison works correctly
	// Note: We can't easily test the timing properties in a unit test
	
	testKey := "sk-oai-test123"
	
	// Create two salted hashes of the same key
	hash1, err := api_keys.HashAPIKeyWithSalt(testKey)
	if err != nil {
		t.Fatalf("HashAPIKeyWithSalt() returned error: %v", err)
	}
	
	hash2, err := api_keys.HashAPIKeyWithSalt(testKey)
	if err != nil {
		t.Fatalf("HashAPIKeyWithSalt() returned error: %v", err)
	}
	
	// Both should validate the same key
	if !api_keys.ValidateAPIKeyHash(testKey, hash1) {
		t.Error("ValidateAPIKeyHash() should validate first hash")
	}
	
	if !api_keys.ValidateAPIKeyHash(testKey, hash2) {
		t.Error("ValidateAPIKeyHash() should validate second hash")
	}
	
	// Cross validation should fail (different salts)
	if api_keys.ValidateAPIKeyHash("sk-oai-different", hash1) {
		t.Error("ValidateAPIKeyHash() should reject wrong key")
	}
}

func TestIsValidKeyFormat(t *testing.T) {
	t.Run("ValidKey", func(t *testing.T) {
		if !api_keys.IsValidKeyFormat("sk-oai-ABC123xyz") {
			t.Error("Valid key should pass")
		}
	})

	t.Run("InvalidKey", func(t *testing.T) {
		if api_keys.IsValidKeyFormat("invalid-key") {
			t.Error("Invalid key should fail")
		}
	})
}

func BenchmarkGenerateAPIKey(b *testing.B) {
	for b.Loop() {
		_, _, _, _ = api_keys.GenerateAPIKey()
	}
}

func BenchmarkHashAPIKeyWithSalt(b *testing.B) {
	key := "sk-oai-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh"

	for b.Loop() {
		_, _ = api_keys.HashAPIKeyWithSalt(key)
	}
}

func BenchmarkValidateAPIKeyHash(b *testing.B) {
	key := "sk-oai-0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefgh"
	
	// Create a salted hash for benchmark
	saltedHash, err := api_keys.HashAPIKeyWithSalt(key)
	if err != nil {
		b.Fatalf("HashAPIKeyWithSalt() returned error: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		_ = api_keys.ValidateAPIKeyHash(key, saltedHash)
	}
}
