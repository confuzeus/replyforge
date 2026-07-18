package sanitizer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizer_Sanitize(t *testing.T) {
	s := NewSanitizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "strips script tags including content",
			input:    "<script>alert('xss')</script>",
			expected: "",
		},
		{
			name:     "strips multiple HTML tags preserving text",
			input:    "<b>Bold</b> and <i>Italic</i>",
			expected: "Bold and Italic",
		},
		{
			name:     "strips attributes from tags preserving text",
			input:    `<a href="http://evil.com" onclick="steal()">Click</a>`,
			expected: "Click",
		},
		{
			name:     "trims leading and trailing whitespace",
			input:    "  Hello World  ",
			expected: "Hello World",
		},
		{
			name:     "collapses multiple spaces",
			input:    "Hello    World",
			expected: "Hello World",
		},
		{
			name:     "collapses tabs and newlines",
			input:    "Hello\t\tWorld\n\nTest",
			expected: "Hello World Test",
		},
		{
			name:     "removes null bytes",
			input:    "Hello\x00World",
			expected: "HelloWorld",
		},
		{
			name:     "preserves valid unicode",
			input:    "Jöhn Döe こんにちは",
			expected: "Jöhn Döe こんにちは",
		},
		{
			name:     "NFC normalizes combining characters",
			input:    "e\u0301", // e + combining acute accent (NFD)
			expected: "\u00e9",  // é precomposed (NFC)
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \t\n   ",
			expected: "",
		},
		{
			name:     "plain text passes through",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "idempotent - strips twice",
			input:    "<script>alert('xss')</script>",
			expected: "",
		},
		{
			name:     "idempotent - whitespace twice",
			input:    "  Hello   World  ",
			expected: "Hello World",
		},
		{
			name:     "idempotent - null bytes twice",
			input:    "Hello\x00World\x00",
			expected: "HelloWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizer_Idempotent(t *testing.T) {
	s := NewSanitizer()

	inputs := []string{
		"<script>alert('xss')</script>",
		"  Hello   World  ",
		"Hello\x00World",
		"Jöhn <b>Döe</b>",
		"",
	}

	for _, input := range inputs {
		first := s.Sanitize(input)
		second := s.Sanitize(first)
		assert.Equal(t, first, second, "Sanitize should be idempotent for input: %q", input)
	}
}

func TestSanitizer_MultipleNullBytes(t *testing.T) {
	s := NewSanitizer()
	result := s.Sanitize("A\x00B\x00C")
	assert.Equal(t, "ABC", result)
}

func TestSanitizer_SurrogateNullBytes(t *testing.T) {
	s := NewSanitizer()
	input := strings.Repeat("\x00", 10)
	result := s.Sanitize(input)
	assert.Equal(t, "", result)
}

func TestSanitizer_MixedAttackVector(t *testing.T) {
	s := NewSanitizer()
	input := "\t\n  <script>alert('xss')</script>  \x00<b>bold</b>\n\t"
	result := s.Sanitize(input)
	assert.Equal(t, "bold", result)
}
