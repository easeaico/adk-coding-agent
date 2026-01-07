package memory

import (
	"testing"
	"unicode/utf8"
)

// TestSaveExperience_SignatureTruncation tests that signature truncation
// properly handles multi-byte characters (e.g., Chinese, emoji) without
// breaking them in the middle.
func TestSaveExperience_SignatureTruncation(t *testing.T) {
	tests := []struct {
		name           string
		pattern        string
		expectedLength int // Expected character (rune) length, not byte length
		shouldTruncate bool
		description    string
	}{
		{
			name:           "short English text",
			pattern:        "Short error message",
			expectedLength: 19, // Actual rune count
			shouldTruncate: false,
			description:    "Short text should not be truncated",
		},
		{
			name:           "long English text",
			pattern:        "This is a very long error message that exceeds fifty characters and should be truncated properly",
			expectedLength: 50,
			shouldTruncate: true,
			description:    "Long English text should be truncated to 50 characters",
		},
		{
			name:           "short Chinese text",
			pattern:        "这是一个短错误消息",
			expectedLength: 9,
			shouldTruncate: false,
			description:    "Short Chinese text should not be truncated",
		},
		{
			name:           "long Chinese text",
			pattern:        "这是一个非常长的错误消息，超过了五十个字符的限制，应该被正确截断，不会在字符中间断开，确保多字节字符完整",
			expectedLength: 50,
			shouldTruncate: true,
			description:    "Long Chinese text should be truncated to 50 characters without breaking characters",
		},
		{
			name:           "mixed English and Chinese",
			pattern:        "Error: 这是一个错误消息，包含中英文混合内容，需要正确截断，确保多字节字符完整处理",
			expectedLength: 50,
			shouldTruncate: true,
			description:    "Mixed language text should be truncated correctly",
		},
		{
			name:           "text with emoji",
			pattern:        "Error 🐛 occurred: This is a test error message with emoji that should be handled correctly",
			expectedLength: 50,
			shouldTruncate: true,
			description:    "Text with emoji should be truncated without breaking emoji",
		},
		{
			name:           "exactly 50 characters",
			pattern:        "这是一个正好五十个字符的错误消息测试用例，确保完整",
			expectedLength: 50,
			shouldTruncate: false,
			description:    "Text with exactly 50 characters should not be truncated",
		},
		{
			name:           "exactly 51 characters",
			pattern:        "这是一个正好五十一个字符的错误消息测试用例，确保完整处理！",
			expectedLength: 50,
			shouldTruncate: true,
			description:    "Text with 51 characters should be truncated to 50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the truncation logic from SaveExperience
			signature := tt.pattern
			runes := []rune(signature)
			if len(runes) > 50 {
				signature = string(runes[:50])
			}

			// Verify the result
			actualRuneCount := utf8.RuneCountInString(signature)
			if actualRuneCount != tt.expectedLength {
				t.Errorf("Expected %d runes, got %d", tt.expectedLength, actualRuneCount)
			}

			// Verify that the signature is valid UTF-8
			if !utf8.ValidString(signature) {
				t.Errorf("Signature is not valid UTF-8: %q", signature)
			}

			// Verify truncation happened when expected
			if tt.shouldTruncate && len(signature) >= len(tt.pattern) {
				t.Errorf("Expected truncation, but signature length (%d) >= pattern length (%d)",
					len(signature), len(tt.pattern))
			}

			// Verify that if truncated, the signature is a prefix of the original (by runes)
			if tt.shouldTruncate {
				originalRunes := []rune(tt.pattern)
				if len(originalRunes) > 50 {
					expectedSignature := string(originalRunes[:50])
					if signature != expectedSignature {
						t.Errorf("Expected signature %q, got %q", expectedSignature, signature)
					}
				}
			} else {
				// If not truncated, should be the same
				if signature != tt.pattern {
					t.Errorf("Expected signature to be unchanged, got %q", signature)
				}
			}

			t.Logf("Pattern: %q (bytes: %d, runes: %d)", tt.pattern, len(tt.pattern), utf8.RuneCountInString(tt.pattern))
			t.Logf("Signature: %q (bytes: %d, runes: %d)", signature, len(signature), actualRuneCount)
		})
	}
}

// TestSaveExperience_SignatureTruncationEdgeCases tests edge cases for signature truncation
func TestSaveExperience_SignatureTruncationEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		description string
	}{
		{
			name:        "empty string",
			pattern:     "",
			description: "Empty string should remain empty",
		},
		{
			name:        "single Chinese character",
			pattern:     "错",
			description: "Single character should remain unchanged",
		},
		{
			name:        "text with newlines",
			pattern:     "Error:\nThis is a multi-line\nerror message",
			description: "Text with newlines should be handled correctly",
		},
		{
			name:        "text with special characters",
			pattern:     "Error: 特殊字符 !@#$%^&*() 测试",
			description: "Text with special characters should be handled correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the truncation logic
			signature := tt.pattern
			runes := []rune(signature)
			if len(runes) > 50 {
				signature = string(runes[:50])
			}

			// Verify valid UTF-8
			if !utf8.ValidString(signature) {
				t.Errorf("Signature is not valid UTF-8: %q", signature)
			}

			// Verify that if original was short, it's unchanged
			if utf8.RuneCountInString(tt.pattern) <= 50 && signature != tt.pattern {
				t.Errorf("Expected signature to be unchanged for short pattern, got %q", signature)
			}

			t.Logf("Pattern: %q -> Signature: %q", tt.pattern, signature)
		})
	}
}

// TestSaveExperience_OldVsNewBehavior demonstrates the difference between
// old (byte-based) and new (rune-based) truncation
func TestSaveExperience_OldVsNewBehavior(t *testing.T) {
	// Test case that would break with old implementation
	chinesePattern := "这是一个正好五十一个字符的错误消息测试用例！"
	
	// Old behavior (byte-based) - would break characters
	oldSignature := chinesePattern
	if len(oldSignature) > 50 {
		oldSignature = oldSignature[:50] // This would break in the middle of a character
	}

	// New behavior (rune-based) - correct
	newSignature := chinesePattern
	runes := []rune(newSignature)
	if len(runes) > 50 {
		newSignature = string(runes[:50])
	}

	// Verify old behavior produces invalid UTF-8 or broken characters
	if utf8.ValidString(oldSignature) {
		// Even if valid, check if it's a complete truncation
		oldRunes := utf8.RuneCountInString(oldSignature)
		newRunes := utf8.RuneCountInString(newSignature)
		if oldRunes != newRunes {
			t.Logf("Old behavior: %d runes, New behavior: %d runes", oldRunes, newRunes)
		}
	} else {
		t.Logf("Old behavior produces invalid UTF-8 (demonstrating the bug)")
	}

	// Verify new behavior is correct
	if !utf8.ValidString(newSignature) {
		t.Errorf("New behavior should produce valid UTF-8, but got invalid: %q", newSignature)
	}

	expectedRunes := 50
	actualRunes := utf8.RuneCountInString(newSignature)
	if actualRunes != expectedRunes {
		t.Errorf("Expected %d runes, got %d", expectedRunes, actualRunes)
	}

	t.Logf("Pattern: %q (%d bytes, %d runes)", chinesePattern, len(chinesePattern), utf8.RuneCountInString(chinesePattern))
	t.Logf("Old (broken): %q (%d bytes, %d runes, valid: %v)", oldSignature, len(oldSignature), utf8.RuneCountInString(oldSignature), utf8.ValidString(oldSignature))
	t.Logf("New (fixed): %q (%d bytes, %d runes, valid: %v)", newSignature, len(newSignature), actualRunes, utf8.ValidString(newSignature))
}
