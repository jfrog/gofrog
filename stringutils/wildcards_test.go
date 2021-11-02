package stringutils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMatchWildcardPattern(t *testing.T) {
	tests := []struct {
		pattern         string
		str             string
		expectedMatched bool
		expectError     bool
	}{
		{"abc", "abc", true, false},
		{"abc", "abcd", false, false},
		{"abc", "ab", false, false},
		{"abc*", "abc", true, false},
		{"abc*", "abcd", true, false},
		{"abc*fg", "abcdefg", true, false},
		{"abc*fg", "abdefg", false, false},
		{"a*c*fg", "abcdefg", true, false},
		{"a*c*fg", "abdefg", false, false},
		{"a*[c", "ab[c", true, false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("pattern: %s, str: %s", tc.pattern, tc.str), func(t *testing.T) {
			actualMatched, err := MatchWildcardPattern(tc.pattern, tc.str)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedMatched, actualMatched)
		})
	}
}
