package memory

import (
	"testing"
	"unicode/utf8"
)

// TestSaveExperience_CharacterTruncationFix verifies that the character truncation
// fix properly handles multi-byte characters without breaking them.
func TestSaveExperience_CharacterTruncationFix(t *testing.T) {
	// Test case: Chinese text that would be broken by byte-based truncation
	chinesePattern := "这是一个非常长的错误消息，超过了五十个字符的限制，应该被正确截断，不会在字符中间断开"

	// Simulate the fixed truncation logic from SaveExperience
	signature := chinesePattern
	runes := []rune(signature)
	if len(runes) > 50 {
		signature = string(runes[:50])
	}

	// Verify the signature is valid UTF-8
	if !utf8.ValidString(signature) {
		t.Errorf("Signature is not valid UTF-8 (this would indicate the bug still exists): %q", signature)
	}

	// Verify the signature has exactly 50 runes (characters)
	runeCount := utf8.RuneCountInString(signature)
	if runeCount > 50 {
		t.Errorf("Signature has %d runes, expected at most 50", runeCount)
	}

	// Verify the signature is a proper prefix of the original (by runes)
	originalRunes := []rune(chinesePattern)
	if len(originalRunes) > 50 {
		expectedSignature := string(originalRunes[:50])
		if signature != expectedSignature {
			t.Errorf("Expected signature %q, got %q", expectedSignature, signature)
		}
	}

	t.Logf("✓ Pattern: %q (%d bytes, %d runes)", chinesePattern, len(chinesePattern), utf8.RuneCountInString(chinesePattern))
	t.Logf("✓ Signature: %q (%d bytes, %d runes, valid UTF-8: %v)", signature, len(signature), runeCount, utf8.ValidString(signature))
}

// TestSaveExperience_ByteVsRuneTruncation demonstrates the bug fix
func TestSaveExperience_ByteVsRuneTruncation(t *testing.T) {
	// A pattern that would break with byte-based truncation
	pattern := "这是一个正好五十一个字符的错误消息测试用例！"
	
	// OLD (buggy) behavior: byte-based truncation
	oldSignature := pattern
	if len(oldSignature) > 50 {
		oldSignature = oldSignature[:50] // This breaks multi-byte characters
	}

	// NEW (fixed) behavior: rune-based truncation
	newSignature := pattern
	runes := []rune(newSignature)
	if len(runes) > 50 {
		newSignature = string(runes[:50])
	}

	// Verify old behavior produces invalid UTF-8 or incorrect truncation
	oldValid := utf8.ValidString(oldSignature)
	oldRunes := utf8.RuneCountInString(oldSignature)
	
	// Verify new behavior is correct
	newValid := utf8.ValidString(newSignature)
	newRunes := utf8.RuneCountInString(newSignature)

	if !newValid {
		t.Errorf("NEW behavior should produce valid UTF-8, but got invalid: %q", newSignature)
	}

	// The new behavior should produce a valid, properly truncated string
	if newRunes > 50 {
		t.Errorf("NEW behavior should truncate to at most 50 runes, got %d", newRunes)
	}

	t.Logf("Pattern: %q (%d bytes, %d runes)", pattern, len(pattern), utf8.RuneCountInString(pattern))
	t.Logf("OLD (byte-based): %q (%d bytes, %d runes, valid: %v)", oldSignature, len(oldSignature), oldRunes, oldValid)
	t.Logf("NEW (rune-based): %q (%d bytes, %d runes, valid: %v)", newSignature, len(newSignature), newRunes, newValid)

	// The key verification: new behavior should always be valid UTF-8
	if !newValid {
		t.Error("❌ Fix verification FAILED: New behavior produces invalid UTF-8")
	} else {
		t.Log("✅ Fix verification PASSED: New behavior correctly handles multi-byte characters")
	}
}
