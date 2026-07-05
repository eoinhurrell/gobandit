package experiment

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

// KeyGenerator interface for generating unique keys
type KeyGenerator interface {
	GenerateExperimentKey(name string) string
	GenerateVariantKey(experimentKey, name string) string
	ValidateKey(key string) error
}

// DefaultKeyGenerator implements KeyGenerator with deterministic and conflict resolution
type DefaultKeyGenerator struct{}

// NewKeyGenerator creates a new key generator
func NewKeyGenerator() KeyGenerator {
	return &DefaultKeyGenerator{}
}

// GenerateExperimentKey generates a URL-safe key for an experiment
func (g *DefaultKeyGenerator) GenerateExperimentKey(name string) string {
	base := g.sanitizeForKey(name)
	if base == "" {
		base = "experiment"
	}

	// Add timestamp suffix for uniqueness
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s_%d", base, timestamp)
}

// GenerateVariantKey generates a URL-safe key for a variant within an experiment
func (g *DefaultKeyGenerator) GenerateVariantKey(experimentKey, name string) string {
	base := g.sanitizeForKey(name)
	if base == "" {
		base = "variant"
	}

	// Add random suffix to ensure uniqueness within experiment
	suffix := g.generateRandomSuffix()
	return fmt.Sprintf("%s_%s", base, suffix)
}

// ValidateKey validates that a key meets the format requirements
func (g *DefaultKeyGenerator) ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	if len(key) > 255 {
		return fmt.Errorf("key cannot be longer than 255 characters")
	}

	// Must be URL-safe: alphanumeric, underscore, hyphen
	validKeyRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !validKeyRegex.MatchString(key) {
		return fmt.Errorf("key must contain only alphanumeric characters, underscores, and hyphens")
	}

	// Cannot start or end with underscore or hyphen
	if strings.HasPrefix(key, "_") || strings.HasPrefix(key, "-") ||
		strings.HasSuffix(key, "_") || strings.HasSuffix(key, "-") {
		return fmt.Errorf("key cannot start or end with underscore or hyphen")
	}

	return nil
}

// sanitizeForKey converts a name to a URL-safe key format
func (g *DefaultKeyGenerator) sanitizeForKey(name string) string {
	// Convert to lowercase
	result := strings.ToLower(name)

	// Replace spaces and other characters with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	result = reg.ReplaceAllString(result, "_")

	// Remove leading/trailing underscores
	result = strings.Trim(result, "_")

	// Limit length
	if len(result) > 50 {
		result = result[:50]
		result = strings.TrimSuffix(result, "_")
	}

	return result
}

// generateRandomSuffix generates a random suffix for uniqueness
func (g *DefaultKeyGenerator) generateRandomSuffix() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 6

	result := make([]byte, length)
	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[num.Int64()]
	}
	return string(result)
}

// ConflictResolver handles key conflicts by generating alternatives
type ConflictResolver struct {
	keyGen KeyGenerator
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(keyGen KeyGenerator) *ConflictResolver {
	return &ConflictResolver{keyGen: keyGen}
}

// ResolveExperimentKeyConflict generates an alternative key when there's a conflict
func (cr *ConflictResolver) ResolveExperimentKeyConflict(originalKey string, attempt int) string {
	// Add numeric suffix to resolve conflict
	return fmt.Sprintf("%s_%d", originalKey, attempt)
}

// ResolveVariantKeyConflict generates an alternative variant key when there's a conflict
func (cr *ConflictResolver) ResolveVariantKeyConflict(originalKey string, attempt int) string {
	// Add numeric suffix to resolve conflict
	return fmt.Sprintf("%s_%d", originalKey, attempt)
}
