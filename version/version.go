package version

import (
	"strconv"
	"strings"
)

type Version struct {
	version string
}

// Index positions x.y.z
const (
	Patch = 2
	Minor = 1
	Major = 0
)

func NewVersion(version string) *Version {
	return &Version{version: version}
}

func (v *Version) GetVersion() string {
	return v.version
}

func (v *Version) SetVersion(version string) {
	v.version = version
}

// If ver1 == version returns 0
// If ver1 > version returns 1
// If ver1 < version returns -1
func (v *Version) Compare(ver1 string) int {
	if ver1 == v.version {
		return 0
	} else if ver1 == "development" {
		return 1
	} else if v.version == "development" {
		return -1
	}

	ver1Tokens := strings.Split(ver1, ".")
	ver2Tokens := strings.Split(v.version, ".")

	maxIndex := len(ver1Tokens)
	if len(ver2Tokens) > maxIndex {
		maxIndex = len(ver2Tokens)
	}

	for tokenIndex := 0; tokenIndex < maxIndex; tokenIndex++ {
		ver1Token := "0"
		if len(ver1Tokens) >= tokenIndex+1 {
			ver1Token = strings.TrimSpace(ver1Tokens[tokenIndex])
		}
		ver2Token := "0"
		if len(ver2Tokens) >= tokenIndex+1 {
			ver2Token = strings.TrimSpace(ver2Tokens[tokenIndex])
		}
		compare := compareTokens(ver1Token, ver2Token)
		if compare != 0 {
			return compare
		}
	}

	return 0
}

// Returns true if this version is larger or equals from the version sent as an argument.
func (v *Version) AtLeast(minVersion string) bool {
	return v.Compare(minVersion) <= 0
}

// Check if candidate is an upgrade/downgrade of the base version only in the specified position
// which corresponds to Major, Minor and Patch positions.
// If True, returns the version, if false return nil.
// Examples: currVersion = Version{"1.2.1}
// currVersion.IsUpgradeAtPosition(Version{"1.2.2"}, 3) => Version{"1.2.2"}.
// currVersion.IsUpgradeAtPosition(Version{"1.2.0"}, 3) => nil.
// currVersion.IsUpgradeAtPosition(Version{"2.2.0"}, 1) => Version{"2.2.0"}.
func (v *Version) IsUpgradeAtPosition(candidate Version, semanticPosition int) (*Version, error) {
	return checkUpgradeAtIndex(v.version, candidate.version, semanticPosition, func(v1Part int, v2Part int) bool {
		return v1Part < v2Part
	})
}

// Examples: currVersion = Version{"1.2.1}.
// currVersion.IsDowngradeAtPosition(Version{"1.2.0"}, 3) => Version{"1.2.0"}.
// currVersion.IsDowngradeAtPosition(Version{"1.2.5"}, 3) => nil.
// currVersion.IsDowngradeAtPosition(Version{"0.2.0"}, 1) => Version{"0.2.0"}.
func (v *Version) IsDowngradeAtPosition(candidate Version, semanticPosition int) (*Version, error) {
	return checkUpgradeAtIndex(v.version, candidate.version, semanticPosition, func(v1Part int, v2Part int) bool {
		return v1Part > v2Part
	})
}

// Helper function to check whether a version is an upgrade or a downgrade of another version at the specified position.
func checkUpgradeAtIndex(v1 string, v2 string, semanticPosition int, compareFunc func(int, int) bool) (*Version, error) {
	v1Parts := strings.Split(strings.Trim(v1, "v"), ".")
	v2Parts := strings.Split(strings.Trim(v2, "v"), ".")
	for i := 0; i < 3; i++ {
		v1Part, err := strconv.Atoi(v1Parts[i])
		if err != nil {
			return nil, err
		}
		v2Part, err := strconv.Atoi(v2Parts[i])
		if err != nil {
			return nil, err
		}
		if v1Part == v2Part {
			continue
		}
		// Verify we are at the specific semanticPosition we want to check
		if i < semanticPosition {
			return nil, nil
		}
		if compareFunc(v1Part, v2Part) {
			return &Version{version: v2}, nil
		} else {
			return nil, nil
		}
	}
	return nil, nil
}

func compareTokens(ver1Token, ver2Token string) int {
	if ver1Token == ver2Token {
		return 0
	}

	// Ignoring error because we strip all the non numeric values in advance.
	ver1Number, ver1Suffix := splitNumberAndSuffix(ver1Token)
	ver1TokenInt, _ := strconv.Atoi(ver1Number)
	ver2Number, ver2Suffix := splitNumberAndSuffix(ver2Token)
	ver2TokenInt, _ := strconv.Atoi(ver2Number)

	switch {
	case ver1TokenInt > ver2TokenInt:
		return 1
	case ver1TokenInt < ver2TokenInt:
		return -1
	case len(ver1Suffix) == 0: // Version with suffix is higher than the same version without suffix
		return -1
	case len(ver2Suffix) == 0:
		return 1
	default:
		return strings.Compare(ver1Token, ver2Token)
	}
}

func splitNumberAndSuffix(token string) (string, string) {
	numeric := ""
	var i int
	for i = 0; i < len(token); i++ {
		n := token[i : i+1]
		if _, err := strconv.Atoi(n); err != nil {
			break
		}
		numeric += n
	}
	if len(numeric) == 0 {
		return "0", token
	}
	return numeric, token[i:]
}
