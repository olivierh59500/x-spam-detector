package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidAPIKey     = errors.New("invalid API key")
	ErrAPIKeyExpired     = errors.New("API key expired")
	ErrAPIKeyNotFound    = errors.New("API key not found")
	ErrInvalidKeyFormat  = errors.New("invalid API key format")
	ErrWeakKey          = errors.New("API key too weak")
)

// APIKeyConfig holds configuration for API key security
type APIKeyConfig struct {
	RequireAuth     bool          `json:"require_auth" yaml:"require_auth"`
	KeyLength       int           `json:"key_length" yaml:"key_length"`
	HashIterations  int           `json:"hash_iterations" yaml:"hash_iterations"`
	KeyExpiration   time.Duration `json:"key_expiration" yaml:"key_expiration"`
	EnableRotation  bool          `json:"enable_rotation" yaml:"enable_rotation"`
	RotationInterval time.Duration `json:"rotation_interval" yaml:"rotation_interval"`
}

// DefaultAPIKeyConfig returns secure default configuration
func DefaultAPIKeyConfig() APIKeyConfig {
	return APIKeyConfig{
		RequireAuth:      true,
		KeyLength:        32,
		HashIterations:   10000,
		KeyExpiration:    24 * time.Hour,
		EnableRotation:   true,
		RotationInterval: 12 * time.Hour,
	}
}

// APIKey represents a secure API key with metadata
type APIKey struct {
	ID          string    `json:"id"`
	HashedKey   string    `json:"hashed_key"`
	Salt        string    `json:"salt"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastUsed    time.Time `json:"last_used"`
	UsageCount  int64     `json:"usage_count"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
	IsActive    bool      `json:"is_active"`
}

// APIKeyManager manages API keys securely
type APIKeyManager struct {
	config APIKeyConfig
	keys   map[string]*APIKey
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager(config APIKeyConfig) *APIKeyManager {
	return &APIKeyManager{
		config: config,
		keys:   make(map[string]*APIKey),
	}
}

// GenerateAPIKey generates a new secure API key
func (akm *APIKeyManager) GenerateAPIKey(description string, permissions []string) (*APIKey, string, error) {
	// Generate random key
	keyBytes := make([]byte, akm.config.KeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate random key: %w", err)
	}
	
	// Generate salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, "", fmt.Errorf("failed to generate salt: %w", err)
	}
	
	// Create raw key string
	rawKey := base64.URLEncoding.EncodeToString(keyBytes)
	
	// Validate key strength
	if err := akm.validateKeyStrength(rawKey); err != nil {
		return nil, "", err
	}
	
	// Hash the key with salt
	hashedKey := akm.hashKeyWithSalt(rawKey, salt)
	
	// Create API key object
	apiKey := &APIKey{
		ID:          akm.generateKeyID(),
		HashedKey:   hashedKey,
		Salt:        base64.StdEncoding.EncodeToString(salt),
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(akm.config.KeyExpiration),
		Description: description,
		Permissions: permissions,
		IsActive:    true,
	}
	
	// Store the key
	akm.keys[apiKey.ID] = apiKey
	
	// Return the raw key for the user (only shown once)
	return apiKey, rawKey, nil
}

// ValidateAPIKey validates an API key using secure comparison
func (akm *APIKeyManager) ValidateAPIKey(rawKey string) (*APIKey, error) {
	if !akm.config.RequireAuth {
		// If auth is disabled, return a default key
		return &APIKey{
			ID:          "no-auth",
			Description: "Authentication disabled",
			Permissions: []string{"*"},
			IsActive:    true,
		}, nil
	}
	
	if rawKey == "" {
		return nil, ErrInvalidAPIKey
	}
	
	// Validate key format
	if err := akm.validateKeyFormat(rawKey); err != nil {
		return nil, err
	}
	
	// Check all stored keys
	for _, apiKey := range akm.keys {
		if !apiKey.IsActive {
			continue
		}
		
		// Check expiration
		if time.Now().After(apiKey.ExpiresAt) {
			apiKey.IsActive = false
			continue
		}
		
		// Decode salt
		salt, err := base64.StdEncoding.DecodeString(apiKey.Salt)
		if err != nil {
			continue
		}
		
		// Hash the provided key with the stored salt
		hashedKey := akm.hashKeyWithSalt(rawKey, salt)
		
		// Secure comparison
		if subtle.ConstantTimeCompare([]byte(hashedKey), []byte(apiKey.HashedKey)) == 1 {
			// Update usage statistics
			apiKey.LastUsed = time.Now()
			apiKey.UsageCount++
			
			return apiKey, nil
		}
	}
	
	return nil, ErrInvalidAPIKey
}

// hashKeyWithSalt hashes a key with salt using multiple iterations
func (akm *APIKeyManager) hashKeyWithSalt(key string, salt []byte) string {
	hash := sha256.New()
	hash.Write([]byte(key))
	hash.Write(salt)
	
	result := hash.Sum(nil)
	
	// Multiple iterations for security
	for i := 1; i < akm.config.HashIterations; i++ {
		hash.Reset()
		hash.Write(result)
		hash.Write(salt)
		result = hash.Sum(nil)
	}
	
	return hex.EncodeToString(result)
}

// validateKeyStrength validates that the key meets security requirements
func (akm *APIKeyManager) validateKeyStrength(key string) error {
	if len(key) < 32 {
		return ErrWeakKey
	}
	
	// Check for sufficient entropy (basic check)
	if strings.Count(key, string(key[0])) > len(key)/4 {
		return ErrWeakKey
	}
	
	return nil
}

// validateKeyFormat validates the format of the API key
func (akm *APIKeyManager) validateKeyFormat(key string) error {
	if len(key) == 0 {
		return ErrInvalidKeyFormat
	}
	
	// Check for valid base64 characters
	if _, err := base64.URLEncoding.DecodeString(key); err != nil {
		return ErrInvalidKeyFormat
	}
	
	return nil
}

// generateKeyID generates a unique key ID
func (akm *APIKeyManager) generateKeyID() string {
	idBytes := make([]byte, 8)
	rand.Read(idBytes)
	return "key_" + hex.EncodeToString(idBytes)
}

// RevokeAPIKey revokes an API key
func (akm *APIKeyManager) RevokeAPIKey(keyID string) error {
	if apiKey, exists := akm.keys[keyID]; exists {
		apiKey.IsActive = false
		return nil
	}
	return ErrAPIKeyNotFound
}

// ListAPIKeys returns all API keys (without sensitive data)
func (akm *APIKeyManager) ListAPIKeys() []*APIKey {
	keys := make([]*APIKey, 0, len(akm.keys))
	for _, key := range akm.keys {
		// Return copy without sensitive data
		keyCopy := *key
		keyCopy.HashedKey = "[REDACTED]"
		keyCopy.Salt = "[REDACTED]"
		keys = append(keys, &keyCopy)
	}
	return keys
}

// CleanupExpiredKeys removes expired keys
func (akm *APIKeyManager) CleanupExpiredKeys() {
	now := time.Now()
	for id, key := range akm.keys {
		if now.After(key.ExpiresAt) {
			delete(akm.keys, id)
		}
	}
}

// RotateKeys rotates keys that are due for rotation
func (akm *APIKeyManager) RotateKeys() error {
	if !akm.config.EnableRotation {
		return nil
	}
	
	now := time.Now()
	for _, key := range akm.keys {
		if key.IsActive && now.Sub(key.CreatedAt) > akm.config.RotationInterval {
			// Mark for rotation (in production, you'd notify the user)
			key.ExpiresAt = now.Add(time.Hour) // Grace period
		}
	}
	
	return nil
}

// GetKeyStats returns statistics about API key usage
func (akm *APIKeyManager) GetKeyStats() map[string]interface{} {
	stats := make(map[string]interface{})
	
	totalKeys := len(akm.keys)
	activeKeys := 0
	expiredKeys := 0
	now := time.Now()
	
	for _, key := range akm.keys {
		if key.IsActive && now.Before(key.ExpiresAt) {
			activeKeys++
		} else {
			expiredKeys++
		}
	}
	
	stats["total_keys"] = totalKeys
	stats["active_keys"] = activeKeys
	stats["expired_keys"] = expiredKeys
	stats["auth_required"] = akm.config.RequireAuth
	
	return stats
}

// SetupDefaultKey sets up a default API key if none exists
func (akm *APIKeyManager) SetupDefaultKey() (string, error) {
	if len(akm.keys) > 0 {
		return "", nil // Keys already exist
	}
	
	// Generate a default key for initial setup
	apiKey, rawKey, err := akm.GenerateAPIKey("Default API Key", []string{"*"})
	if err != nil {
		return "", err
	}
	
	// Make it long-lived for initial setup
	apiKey.ExpiresAt = time.Now().Add(365 * 24 * time.Hour)
	
	return rawKey, nil
}