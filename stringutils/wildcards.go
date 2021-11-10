package stringutils

import (
	"regexp"
	"strings"
)

// MatchWildcardPattern returns whether str matches the pattern, which may contain wildcards.
func MatchWildcardPattern(pattern string, str string) (matched bool, err error) {
	regexpPattern := WildcardPatternToRegExp(pattern)
	r, err := regexp.Compile(regexpPattern)
	if err != nil {
		return false, err
	}
	return r.MatchString(str), nil
}

// WildcardPatternToRegExp converts a wildcard pattern to a regular expression.
func WildcardPatternToRegExp(localPath string) string {
	localPath = EscapeSpecialChars(localPath)
	var wildcard = ".*"
	localPath = strings.Replace(localPath, "*", wildcard, -1)
	if strings.HasSuffix(localPath, "/") || strings.HasSuffix(localPath, "\\") {
		localPath += wildcard
	}
	return "^" + localPath + "$"
}

func EscapeSpecialChars(path string) string {
	// We don't replace other special characters (like parenthesis) because they're used in the placeholders logic of the JFrog CLI.
	var specialChars = []string{".", "^", "$", "+"}
	for _, char := range specialChars {
		path = strings.Replace(path, char, "\\"+char, -1)
	}
	return path
}
