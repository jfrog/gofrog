package io

import (
	"bufio"
	"slices"
	"strings"
	"testing"
)

func TestSplitAt(t *testing.T) {
	// Define test cases
	testCases := []struct {
		scenarioDescription string
		inputData           string
		substring           string
		expectedSplits      []string
	}{
		{
			scenarioDescription: "Empty data",
			inputData:           "",
			substring:           "separator",
			expectedSplits:      []string{},
		},
		{
			scenarioDescription: "Data does not contain the separator",
			inputData:           "someThing Without a matching SePaRaToR",
			substring:           "separator",
			expectedSplits:      []string{"someThing Without a matching SePaRaToR"},
		},
		{
			scenarioDescription: "Data contains the separator once",
			inputData:           "AseparatorB",
			substring:           "separator",
			expectedSplits:      []string{"A", "B"},
		},
		{
			scenarioDescription: "Data contains the separator more than once",
			inputData:           "AseparatorBseparatorC",
			substring:           "separator",
			expectedSplits:      []string{"A", "B", "C"},
		},
	}

	// Run test cases
	for _, tc := range testCases {
		scanner := bufio.NewScanner(strings.NewReader(tc.inputData))
		scanner.Split(SplitAt(tc.substring))
		actualSplits := []string{}
		for scanner.Scan() {
			actualSplits = append(actualSplits, scanner.Text())
		}
		if !slices.Equal(tc.expectedSplits, actualSplits) {
			t.Errorf("Test failed for scenario: %s, input data: %s, substring: %s\nExpected: %s\nActual: %s", tc.scenarioDescription, tc.inputData, tc.substring, tc.expectedSplits, actualSplits)
		}
	}
}
