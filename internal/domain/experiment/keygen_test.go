package experiment

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultKeyGenerator_GenerateExperimentKey(t *testing.T) {
	keyGen := NewKeyGenerator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple_name",
			input:    "My Test Experiment",
			expected: "my_test_experiment",
		},
		{
			name:     "special_characters",
			input:    "Test@#$%Experiment!",
			expected: "test_experiment",
		},
		{
			name:     "empty_name",
			input:    "",
			expected: "experiment",
		},
		{
			name:     "long_name",
			input:    strings.Repeat("a", 100),
			expected: strings.Repeat("a", 50),
		},
		{
			name:     "numbers_and_letters",
			input:    "Test 123 Experiment",
			expected: "test_123_experiment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := keyGen.GenerateExperimentKey(tt.input)

			// Should contain the expected base
			assert.Contains(t, key, tt.expected, "Key should contain expected base")

			// Should have timestamp suffix
			parts := strings.Split(key, "_")
			assert.Greater(t, len(parts), 1, "Key should have timestamp suffix")

			// Should be URL-safe
			validKeyRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
			assert.True(t, validKeyRegex.MatchString(key), "Key should be URL-safe")

			// Should not be too long
			assert.LessOrEqual(t, len(key), 255, "Key should not exceed 255 characters")
		})
	}
}

func TestDefaultKeyGenerator_GenerateVariantKey(t *testing.T) {
	keyGen := NewKeyGenerator()

	tests := []struct {
		name           string
		experimentKey  string
		variantName    string
		expectedPrefix string
	}{
		{
			name:           "simple_variant",
			experimentKey:  "exp_123",
			variantName:    "Control Group",
			expectedPrefix: "control_group",
		},
		{
			name:           "empty_variant_name",
			experimentKey:  "exp_123",
			variantName:    "",
			expectedPrefix: "variant",
		},
		{
			name:           "special_characters",
			experimentKey:  "exp_123",
			variantName:    "Variant A!@#",
			expectedPrefix: "variant_a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := keyGen.GenerateVariantKey(tt.experimentKey, tt.variantName)

			// Should contain the expected prefix
			assert.True(t, strings.HasPrefix(key, tt.expectedPrefix),
				"Key should start with expected prefix")

			// Should have random suffix
			parts := strings.Split(key, "_")
			assert.Greater(t, len(parts), 1, "Key should have random suffix")
			suffix := parts[len(parts)-1]
			assert.Len(t, suffix, 6, "Random suffix should be 6 characters")

			// Should be URL-safe
			validKeyRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
			assert.True(t, validKeyRegex.MatchString(key), "Key should be URL-safe")
		})
	}
}

func TestDefaultKeyGenerator_ValidateKey(t *testing.T) {
	keyGen := NewKeyGenerator()

	validKeys := []struct {
		name string
		key  string
	}{
		{"simple_key", "experiment_123"},
		{"with_hyphens", "experiment-test-123"},
		{"mixed_case", "ExperimentTest123"},
		{"numbers_only", "123456"},
		{"letters_only", "experimenttest"},
	}

	for _, tt := range validKeys {
		t.Run("valid_"+tt.name, func(t *testing.T) {
			err := keyGen.ValidateKey(tt.key)
			assert.NoError(t, err, "Valid key should pass validation")
		})
	}

	invalidKeys := []struct {
		name        string
		key         string
		expectedErr string
	}{
		{
			name:        "empty_key",
			key:         "",
			expectedErr: "key cannot be empty",
		},
		{
			name:        "too_long",
			key:         strings.Repeat("a", 256),
			expectedErr: "key cannot be longer than 255 characters",
		},
		{
			name:        "special_characters",
			key:         "experiment@test",
			expectedErr: "key must contain only alphanumeric characters",
		},
		{
			name:        "starts_with_underscore",
			key:         "_experiment",
			expectedErr: "key cannot start or end with underscore or hyphen",
		},
		{
			name:        "ends_with_hyphen",
			key:         "experiment-",
			expectedErr: "key cannot start or end with underscore or hyphen",
		},
		{
			name:        "spaces",
			key:         "experiment test",
			expectedErr: "key must contain only alphanumeric characters",
		},
	}

	for _, tt := range invalidKeys {
		t.Run("invalid_"+tt.name, func(t *testing.T) {
			err := keyGen.ValidateKey(tt.key)
			assert.Error(t, err, "Invalid key should fail validation")
			assert.Contains(t, err.Error(), tt.expectedErr, "Error message should match expected")
		})
	}
}

func TestDefaultKeyGenerator_sanitizeForKey(t *testing.T) {
	keyGen := &DefaultKeyGenerator{}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple_text",
			input:    "My Test",
			expected: "my_test",
		},
		{
			name:     "special_characters",
			input:    "Test!@#$%^&*()Experiment",
			expected: "test_experiment",
		},
		{
			name:     "multiple_spaces",
			input:    "Test    Experiment",
			expected: "test_experiment",
		},
		{
			name:     "leading_trailing_spaces",
			input:    "   Test Experiment   ",
			expected: "test_experiment",
		},
		{
			name:     "empty_string",
			input:    "",
			expected: "",
		},
		{
			name:     "numbers_preserved",
			input:    "Test 123 Experiment",
			expected: "test_123_experiment",
		},
		{
			name:     "very_long_text",
			input:    strings.Repeat("Long", 20) + " Test",
			expected: "longlonglonglonglonglonglonglonglonglonglonglonglo", // Truncated to 50 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := keyGen.sanitizeForKey(tt.input)
			assert.Equal(t, tt.expected, result, "Sanitization result should match expected")

			// Result should not exceed 50 characters
			assert.LessOrEqual(t, len(result), 50, "Sanitized key should not exceed 50 characters")

			// Result should not start or end with underscore
			if result != "" {
				assert.False(t, strings.HasPrefix(result, "_"), "Result should not start with underscore")
				assert.False(t, strings.HasSuffix(result, "_"), "Result should not end with underscore")
			}
		})
	}
}

func TestDefaultKeyGenerator_generateRandomSuffix(t *testing.T) {
	keyGen := &DefaultKeyGenerator{}

	// Generate multiple suffixes to test randomness
	suffixes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		suffix := keyGen.generateRandomSuffix()

		// Should be exactly 6 characters
		assert.Len(t, suffix, 6, "Suffix should be 6 characters long")

		// Should contain only valid characters
		validChars := regexp.MustCompile(`^[a-z0-9]+$`)
		assert.True(t, validChars.MatchString(suffix), "Suffix should contain only lowercase letters and numbers")

		suffixes[suffix] = true
	}

	// Should generate different suffixes (very high probability)
	assert.Greater(t, len(suffixes), 80, "Should generate mostly unique suffixes")
}

func TestConflictResolver_ResolveExperimentKeyConflict(t *testing.T) {
	keyGen := NewKeyGenerator()
	resolver := NewConflictResolver(keyGen)

	originalKey := "experiment_test"

	tests := []struct {
		name     string
		attempt  int
		expected string
	}{
		{
			name:     "first_attempt",
			attempt:  1,
			expected: "experiment_test_1",
		},
		{
			name:     "fifth_attempt",
			attempt:  5,
			expected: "experiment_test_5",
		},
		{
			name:     "tenth_attempt",
			attempt:  10,
			expected: "experiment_test_10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.ResolveExperimentKeyConflict(originalKey, tt.attempt)
			assert.Equal(t, tt.expected, result, "Conflict resolution should match expected")
		})
	}
}

func TestConflictResolver_ResolveVariantKeyConflict(t *testing.T) {
	keyGen := NewKeyGenerator()
	resolver := NewConflictResolver(keyGen)

	originalKey := "control_group"

	tests := []struct {
		name     string
		attempt  int
		expected string
	}{
		{
			name:     "first_attempt",
			attempt:  1,
			expected: "control_group_1",
		},
		{
			name:     "third_attempt",
			attempt:  3,
			expected: "control_group_3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolver.ResolveVariantKeyConflict(originalKey, tt.attempt)
			assert.Equal(t, tt.expected, result, "Variant conflict resolution should match expected")
		})
	}
}

func TestKeyGenerator_Integration(t *testing.T) {
	keyGen := NewKeyGenerator()

	// Generate experiment key
	expKey := keyGen.GenerateExperimentKey("My Test Experiment")
	require.NotEmpty(t, expKey, "Experiment key should not be empty")

	// Validate experiment key
	err := keyGen.ValidateKey(expKey)
	assert.NoError(t, err, "Generated experiment key should be valid")

	// Generate variant keys
	variantNames := []string{"Control", "Variant A", "Variant B"}
	variantKeys := make([]string, len(variantNames))

	for i, name := range variantNames {
		variantKeys[i] = keyGen.GenerateVariantKey(expKey, name)
		require.NotEmpty(t, variantKeys[i], "Variant key should not be empty")

		// Validate variant key
		err := keyGen.ValidateKey(variantKeys[i])
		assert.NoError(t, err, "Generated variant key should be valid")
	}

	// All variant keys should be unique
	keySet := make(map[string]bool)
	for _, key := range variantKeys {
		assert.False(t, keySet[key], "Variant keys should be unique")
		keySet[key] = true
	}
}
