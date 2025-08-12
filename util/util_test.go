// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package util

import (
	"testing"
)

func TestASCIIEqualFold(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		t        string
		expected bool
		reason   string
	}{
		// Basic ASCII case folding tests
		{
			name:     "identical strings",
			s:        "hello",
			t:        "hello",
			expected: true,
			reason:   "identical strings should match",
		},
		{
			name:     "simple case difference",
			s:        "Hello",
			t:        "hello",
			expected: true,
			reason:   "ASCII case folding should match H/h",
		},
		{
			name:     "all uppercase vs lowercase",
			s:        "HELLO",
			t:        "hello",
			expected: true,
			reason:   "ASCII case folding should match all cases",
		},
		{
			name:     "mixed case",
			s:        "HeLLo",
			t:        "hEllO",
			expected: true,
			reason:   "mixed case should match after folding",
		},

		// Empty string tests
		{
			name:     "both empty",
			s:        "",
			t:        "",
			expected: true,
			reason:   "empty strings should match",
		},
		{
			name:     "one empty",
			s:        "hello",
			t:        "",
			expected: false,
			reason:   "different length strings should not match",
		},
		{
			name:     "other empty",
			s:        "",
			t:        "hello",
			expected: false,
			reason:   "different length strings should not match",
		},

		// Different content tests
		{
			name:     "different strings same case",
			s:        "hello",
			t:        "world",
			expected: false,
			reason:   "different content should not match",
		},
		{
			name:     "different strings different case",
			s:        "Hello",
			t:        "World",
			expected: false,
			reason:   "different content should not match regardless of case",
		},
		{
			name:     "different length",
			s:        "hello",
			t:        "hello world",
			expected: false,
			reason:   "different length strings should not match",
		},

		// ASCII non-alphabetic characters
		{
			name:     "numbers and symbols",
			s:        "Hello123!@#",
			t:        "hello123!@#",
			expected: true,
			reason:   "numbers and symbols should be preserved, only letters folded",
		},
		{
			name:     "different numbers",
			s:        "Hello123",
			t:        "Hello124",
			expected: false,
			reason:   "different numbers should not match",
		},
		{
			name:     "different symbols",
			s:        "Hello!",
			t:        "Hello?",
			expected: false,
			reason:   "different symbols should not match",
		},

		// URL-specific tests (CORS security focus)
		{
			name:     "HTTP scheme case",
			s:        "HTTP://example.com",
			t:        "http://example.com",
			expected: true,
			reason:   "HTTP scheme should be case-insensitive",
		},
		{
			name:     "HTTPS scheme case",
			s:        "HTTPS://EXAMPLE.COM",
			t:        "https://example.com",
			expected: true,
			reason:   "HTTPS scheme and domain should be case-insensitive",
		},
		{
			name:     "complex URL case",
			s:        "HTTPS://API.EXAMPLE.COM:8080/PATH",
			t:        "https://api.example.com:8080/path",
			expected: true,
			reason:   "entire URL should be case-insensitive for ASCII",
		},
		{
			name:     "subdomain case",
			s:        "https://API.SUB.EXAMPLE.COM",
			t:        "https://api.sub.example.com",
			expected: true,
			reason:   "subdomains should be case-insensitive",
		},

		// Unicode security tests (homograph attack prevention)
		{
			name:     "cyrillic homograph attack",
			s:        "https://еxample.com", // Cyrillic 'е' (U+0435)
			t:        "https://example.com", // Latin 'e' (U+0065)
			expected: false,
			reason:   "should block Cyrillic homograph attack",
		},
		{
			name:     "mixed cyrillic attack",
			s:        "https://ехample.com", // Cyrillic 'е' and 'х'
			t:        "https://example.com", // Latin 'e' and 'x'
			expected: false,
			reason:   "should block mixed Cyrillic homograph attack",
		},
		{
			name:     "cyrillic 'а' attack",
			s:        "https://exаmple.com", // Cyrillic 'а' (U+0430)
			t:        "https://example.com", // Latin 'a' (U+0061)
			expected: false,
			reason:   "should block Cyrillic 'а' homograph attack",
		},

		// Unicode case folding security tests
		{
			name:     "unicode case folding attack",
			s:        "https://CAFÉ.com", // Latin É (U+00C9)
			t:        "https://café.com", // Latin é (U+00E9)
			expected: false,
			reason:   "should NOT perform Unicode case folding (security)",
		},
		{
			name:     "turkish i attack",
			s:        "https://İSTANBUL.com", // Turkish İ (U+0130)
			t:        "https://istanbul.com", // Latin i
			expected: false,
			reason:   "should NOT perform Turkish case folding",
		},
		{
			name:     "german sharp s",
			s:        "https://GROß.com",  // German ß (U+00DF)
			t:        "https://gross.com", // Expanded form
			expected: false,
			reason:   "should NOT perform German ß expansion",
		},

		// Valid Unicode exact matches
		{
			name:     "identical unicode",
			s:        "https://café.com",
			t:        "https://café.com",
			expected: true,
			reason:   "identical Unicode strings should match",
		},
		{
			name:     "identical cyrillic",
			s:        "https://пример.com", // Russian
			t:        "https://пример.com", // Russian
			expected: true,
			reason:   "identical Cyrillic strings should match",
		},
		{
			name:     "ascii part of unicode domain",
			s:        "HTTPS://café.COM", // ASCII parts should fold
			t:        "https://café.com",
			expected: true,
			reason:   "ASCII parts should fold even in Unicode strings",
		},

		// Edge cases with UTF-8
		{
			name:     "different UTF-8 byte length same rune count",
			s:        "Café", // é is 2 bytes
			t:        "Café", // é is 2 bytes (same)
			expected: true,
			reason:   "same Unicode content should match",
		},
		{
			name:     "UTF-8 normalization difference",
			s:        "café\u0301", // é as e + combining acute (3 bytes for é part)
			t:        "café",       // é as single character (2 bytes for é part)
			expected: false,
			reason:   "different Unicode normalization should not match",
		},
		{
			name:     "CRITICAL: current implementation flaw",
			s:        "ABC" + string([]byte{0xC3, 0xA9}), // ABC + é (2 bytes) = 5 bytes
			t:        "abc" + string([]byte{0xC3, 0xA9}), // abc + é (2 bytes) = 5 bytes
			expected: true,
			reason:   "should match after ASCII folding (this should pass with correct implementation)",
		},
		{
			name:     "invalid UTF-8 sequence",
			s:        "hello\xff", // Invalid UTF-8
			t:        "hello\xff", // Invalid UTF-8
			expected: true,
			reason:   "identical invalid UTF-8 should match",
		},
		{
			name:     "different invalid UTF-8",
			s:        "hello\xff", // Invalid UTF-8
			t:        "hello\xfe", // Different invalid UTF-8
			expected: false,
			reason:   "different invalid UTF-8 should not match",
		},

		// ASCII boundary tests
		{
			name:     "ascii boundary characters",
			s:        "A@Z[`a{z", // Test boundaries around A-Z
			t:        "a@z[`A{Z",
			expected: true,
			reason:   "only A-Z should be folded, not punctuation",
		},
		{
			name:     "digit boundaries",
			s:        "Test123ABC",
			t:        "test123abc",
			expected: true,
			reason:   "digits should not be folded, only letters",
		},

		// Long string performance tests
		{
			name:     "long ascii string",
			s:        "HTTP://" + repeatString("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 100) + ".COM",
			t:        "http://" + repeatString("abcdefghijklmnopqrstuvwxyz", 100) + ".com",
			expected: true,
			reason:   "long ASCII strings should be handled efficiently",
		},
		{
			name:     "long unicode string",
			s:        repeatString("CAFÉ", 100),
			t:        repeatString("CAFÉ", 100), // Same case - should match
			expected: true,
			reason:   "long identical Unicode strings should match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ASCIIEqualFold(tt.s, tt.t)
			if result != tt.expected {
				t.Errorf("ASCIIEqualFold(%q, %q) = %v, expected %v\nReason: %s",
					tt.s, tt.t, result, tt.expected, tt.reason)
			}
		})
	}
}

// Helper function for generating long test strings
func repeatString(s string, count int) string {
	if count <= 0 {
		return ""
	}
	result := make([]byte, 0, len(s)*count)
	for i := 0; i < count; i++ {
		result = append(result, s...)
	}
	return string(result)
}

// Benchmark tests for performance verification
func BenchmarkASCIIEqualFold(b *testing.B) {
	benchmarks := []struct {
		name string
		s    string
		t    string
	}{
		{
			name: "short_ascii_match",
			s:    "HTTP://EXAMPLE.COM",
			t:    "http://example.com",
		},
		{
			name: "short_ascii_nomatch",
			s:    "HTTP://EXAMPLE.COM",
			t:    "http://different.com",
		},
		{
			name: "long_ascii_match",
			s:    "HTTP://" + repeatString("ABCDEFGHIJKLMNOPQRSTUVWXYZ", 100) + ".COM",
			t:    "http://" + repeatString("abcdefghijklmnopqrstuvwxyz", 100) + ".com",
		},
		{
			name: "unicode_nomatch",
			s:    "https://café.com",
			t:    "https://CAFÉ.com",
		},
		{
			name: "unicode_exact_match",
			s:    "https://café.com",
			t:    "https://café.com",
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ASCIIEqualFold(bm.s, bm.t)
			}
		})
	}
}

// Fuzzing test to catch edge cases
func FuzzASCIIEqualFold(f *testing.F) {
	// Seed with interesting test cases
	seeds := [][]string{
		{"hello", "HELLO"},
		{"", ""},
		{"café", "CAFÉ"},
		{"https://example.com", "HTTPS://EXAMPLE.COM"},
		{"еxample", "example"},                       // Cyrillic attack
		{string([]byte{0xff}), string([]byte{0xfe})}, // Invalid UTF-8
	}

	for _, seed := range seeds {
		f.Add(seed[0], seed[1])
	}

	f.Fuzz(func(t *testing.T, s1, s2 string) {
		// Just ensure it doesn't panic and returns a boolean
		result := ASCIIEqualFold(s1, s2)
		_ = result // Use the result to prevent optimization

		// Property: function should be symmetric
		if ASCIIEqualFold(s1, s2) != ASCIIEqualFold(s2, s1) {
			t.Errorf("ASCIIEqualFold is not symmetric: (%q, %q)", s1, s2)
		}

		// Property: identical strings should always match
		if s1 == s2 && !ASCIIEqualFold(s1, s2) {
			t.Errorf("identical strings should match: %q", s1)
		}
	})
}
