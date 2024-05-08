package stringMap

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func FromStruct(s any) map[string]string {
	if s == nil {
		return map[string]string{}
	}
	var (
		m        = map[string]string{}
		v        = reflect.ValueOf(s)
		vT       = v.Type()
		vF       reflect.StructField
		jsonData []byte
	)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		vT = vT.Elem()
	}
	for i, l := 0, vT.NumField(); i < l; i++ {
		vF = vT.Field(i)
		if vF.IsExported() && !vF.Anonymous {
			switch vF.Type.Kind() {
			case reflect.Struct, reflect.Ptr, reflect.Slice, reflect.Map, reflect.Array:
				jsonData, _ = json.Marshal(v.Field(i).Interface())
				m[vF.Name] = string(jsonData)
			case reflect.String:
				m[vF.Name] = v.Field(i).String()
			default:
				m[vF.Name] = fmt.Sprintf("%v", v.Field(i).Interface())
			}
		}
	}
	return m
}

func StructsDelta(s1Map map[string]string, s2 any) map[string]string {
	if s1Map == nil || len(s1Map) == 0 {
		return FromStruct(s2)
	}
	if s2 == nil {
		return map[string]string{}
	}
	var (
		s2Map = FromStruct(s2)
		res   = map[string]string{}
	)
	for k, v := range s1Map {
		if s2Map[k] != v {
			res[k] = v
		}
	}
	return res
}
