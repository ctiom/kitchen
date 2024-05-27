package kitchen

import (
	"fmt"
	"github.com/iancoleman/strcase"
	"reflect"
	"strconv"
	"strings"
)

func ContainsValueInSlice(slice []string, value string) bool {
	for _, v := range slice {
		if strcase.ToCamel(v) == strcase.ToCamel(value) {
			return true
		}
	}
	return false
}

type nameAndPos struct {
	name string
	pos  int
}

func DishUrlAndMethod(dish IDish) (url string, urlParams, queryParams [][2]string, method string, queryParamsRequired []string, webQueryParamMap []nameAndPos, webUrlParamMap []int) {
	var (
		input any
		param string
	)
	input, _ = dish.IO()
	switch input.(type) {
	case string:
		urlParams = append(urlParams, [2]string{"{p1}", ""})
		method = setValueIfEmpty(method, "GET")
	default:
		iType := reflect.TypeOf(input)
		if iType != nil {
			method = setValueIfEmpty(method, "POST")
			if iType.Kind() == reflect.Ptr {
				iType = iType.Elem()
			}
			if iType.Kind() == reflect.Struct {
				for i := 0; i < iType.NumField(); i++ {
					if param = iType.Field(i).Tag.Get("urlParam"); param != "" {
						urlParams = append(urlParams, [2]string{fmt.Sprintf("{%s}", param), iType.Field(i).Tag.Get("desc")})
						webUrlParamMap = append(webUrlParamMap, i)
					} else if param = iType.Field(i).Tag.Get("queryParam"); param != "" {
						queryParams = append(queryParams, [2]string{param, iType.Field(i).Tag.Get("desc")})
						if mandate := iType.Field(i).Tag.Get("required"); mandate != "" {
							mandateValue, err := strconv.ParseBool(mandate)
							if err != nil {
								mandateValue = false
							}
							if mandateValue {
								queryParamsRequired = append(queryParamsRequired, iType.Field(i).Name)
							}
						}
						webQueryParamMap = append(webQueryParamMap, nameAndPos{name: param, pos: i})
					}
				}
			}
		} else {
			method = setValueIfEmpty(method, "GET")
		}
	}
	switch dish.Name() {
	case "GET", "HEAD", "OPTIONS", "PATCH", "TRACE", "CONNECT", "POST", "UPDATE", "DELETE", "PUT":
		method = dish.Name()
	default:
		url = strcase.ToSnake(dish.Name())
	}
	return url, urlParams, queryParams, method, queryParamsRequired, webQueryParamMap, webUrlParamMap
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
