package kitchenWeb

import (
	"github.com/iancoleman/strcase"
	"regexp"
)

func findUrlParamPatterns(input string) []string {
	// Define a regular expression pattern to match strings between curly braces
	re := regexp.MustCompile(`\{([^}]+)\}`)

	// Find all matches in the input string
	matches := re.FindAllString(input, -1)

	return matches
}

func combineAndRemoveDuplicates(arr1, arr2 [][2]string) [][2]string {
	uniqueElements := make(map[string][2]string)

	for _, element := range arr1 {
		uniqueElements[element[0]] = element
	}

	for _, element := range arr2 {
		uniqueElements[element[0]] = element
	}

	var combined [][2]string
	for _, element := range uniqueElements {
		combined = append(combined, element)
	}

	return combined
}

func ContainsValueInSlice(slice []string, value string) bool {
	for _, v := range slice {
		if strcase.ToCamel(v) == strcase.ToCamel(value) {
			return true
		}
	}
	return false
}
