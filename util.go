package kitchen

import (
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
)

func ContainsValueInSlice(slice []string, value string) bool {
	for _, v := range slice {
		if strcase.ToCamel(v) == strcase.ToCamel(value) {
			return true
		}
	}
	return false
}

func findUrlParamPatterns(input string) []string {
	// Define a regular expression pattern to match strings between curly braces
	re := regexp.MustCompile(`\{([^}]+)\}`)

	// Find all matches in the input string
	matches := re.FindAllString(input, -1)

	return matches
}

func combineAndRemoveDuplicates(arr1, arr2 []paramType) []paramType {
	uniqueElements := make(map[string]paramType)

	for _, element := range arr1 {
		uniqueElements[element.Name] = element
	}

	for _, element := range arr2 {
		uniqueElements[element.Name] = element
	}

	var combined []paramType
	for _, element := range uniqueElements {
		combined = append(combined, element)
	}

	return combined
}

func Ternary[T any](condition bool, trueVal, falseVal T) T {
	if condition {
		return trueVal
	}
	return falseVal
}

func setValueIfEmpty(method, value string) string {
	method = strings.Trim(method, " ")
	/*if len(method) == 0 {
		method = value
	}
	return strings.ToUpper(method)*/
	return strings.ToUpper(Ternary[string](len(method) > 0, method, value))
}
