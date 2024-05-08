package kitchen

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
)

type ISwaggerType interface {
	ToSwaggerType(any, map[string]any) (string, []paramType)
}

type ISwaggerRoute interface {
	ToSwaggerRoute(map[string]any, SwaggerOption) map[string]any
}

type hasTypeForExport interface {
	TypeForExport() any
}

type SwaggerOption struct {
	Security         map[string]any
	SecurityMethod   string
	UrlPrefix        string
	SkipWrapperCheck bool
}

type swaggerType struct {
	Name      string
	UrlParams []paramType
	Body      any
}

type paramType struct {
	Name string
	Desc string
}

func NewParamType(name, desc string) paramType {
	return paramType{
		Name: name,
		Desc: desc,
	}
}

func NewParamTypeFromMap(paramtypemap map[string]string) []paramType {
	paramTypes := make([]paramType, 0)
	for k, v := range paramtypemap {
		paramTypes = append(paramTypes, paramType{
			Name: k,
			Desc: v,
		})
	}
	return paramTypes
}

var (
	swaggerTypes = map[reflect.Type]swaggerType{}
)

func RegisterSwaggerType(t reflect.Type, name string, urlParams []paramType, body any) {
	swaggerTypes[t] = swaggerType{
		Name:      name,
		UrlParams: urlParams,
		Body:      body,
	}
}

func MakeSwagger(name string, origin []string, basePath, version string, menu ...IMenu) ([]byte, error) {
	var (
		api     = map[string]any{}
		types   = map[string]any{}
		res     map[string]any
		servers = []map[string]any{}
	)
	for _, o := range origin {
		servers = append(servers, map[string]any{
			"url": o + basePath,
		})
	}
	res = map[string]any{
		"openapi": "3.0.1",
		"servers": servers,
		"info": map[string]any{
			"title":   name,
			"version": version,
		},
		"paths":      api,
		"components": map[string]any{"schemas": types},
	}
	for _, w := range menu {
		opt := w.swaggerOption()

		if opt.Security != nil {
			securities := map[string]any{}
			for k, v := range opt.Security {
				opt.SecurityMethod = k
				securities[k] = v
			}
			res["components"].(map[string]any)["securitySchemes"] = securities
		}
		opt.UrlPrefix = strings.TrimRight(strings.TrimLeft(opt.UrlPrefix, "/"), "/")
		a, t := w.ToSwagger(strings.Split(opt.UrlPrefix, "/"), opt)
		for k, v := range a {
			api[k] = v
		}
		for k, v := range t {
			types[k] = v
		}
	}
	return json.MarshalIndent(res, "", "  ")
}

func (w cookbook[D, I, O]) ToSwagger(prefix []string, options ...SwaggerOption) (api map[string]any, types map[string]any) {
	var (
		ok         bool
		group      iSet[D]
		action     iDish[D]
		option     SwaggerOption
		subTypeKey string
	)
	api = map[string]any{}
	types = map[string]any{}
	if len(prefix) != 0 {
		if prefix[0] == "" {
			prefix = prefix[1:]
		}
	}
	if len(options) == 0 {
		option = SwaggerOption{}
	} else {
		option = options[0]
	}
	for _, node := range w.nodes {
		if group, ok = any(node).(iSet[D]); ok {
			nodeApi, nodeTypes := node.ToSwagger(append(prefix, strcase.ToSnake(group.Name())), options...)
			for k, v := range nodeApi {
				api[k] = v
			}
			for k, v := range nodeTypes {
				types[k] = v
			}
		} else if sr, ok := any(node).(ISwaggerRoute); ok {
			nodeApi := sr.ToSwaggerRoute(types, option)
			for k, v := range nodeApi {
				api["/"+strings.Join(append(prefix, k), "/")] = v
			}
		} else if action, ok = any(node).(iDish[D]); ok {
			var (
				i, o       = action.IO()
				iT, oT     = reflect.TypeOf(i), reflect.TypeOf(o)
				body       = map[string]any{}
				parameters = []map[string]any{}
			)
			url, urlParams, queryParams, method, queryParamsRequired := action.urlAndMethod()
			if url == "" {
				urlParamNames := make([]string, 0)
				for _, up := range urlParams {
					urlParamNames = append(urlParamNames, up.Name)
				}
				url = "/" + strings.Join(append(prefix, urlParamNames...), "/")
			} else {
				urlParamNames := make([]string, 0)
				for _, up := range urlParams {
					urlParamNames = append(urlParamNames, up.Name)
				}
				url = "/" + strings.Join(append(append(prefix, url), urlParamNames...), "/")
			}

			if oT != nil {
				if oT.Kind() == reflect.Ptr {
					oT = oT.Elem()
				}
				var (
					menu    IMenu
					oSchema map[string]any
				)
				if oS, ok := o.(ISwaggerType); ok {
					ref, _ := oS.ToSwaggerType(o, types)
					oSchema = map[string]any{
						"$ref": ref,
					}
					body["responses"] = map[string]any{
						"200": map[string]any{
							"description": "OK",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": oSchema,
								},
							},
						},
					}
				} else if swaggerType, ok := swaggerTypes[oT]; ok {
					types[swaggerType.Name] = swaggerType.Body
					oSchema = map[string]any{
						"$ref": "#/components/schemas/" + swaggerType.Name,
					}
					body["responses"] = map[string]any{
						"200": map[string]any{
							"description": "OK",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": oSchema,
								},
							},
						},
					}
				} else {
					switch oT.Kind() {
					case reflect.Struct:

						subTypeKey, _, _, _ := swaggerParseStruct(oT, types)
						oSchema = map[string]any{
							"$ref": "#/components/schemas/" + subTypeKey,
						}
						if subTypeKey != "" {
							body["responses"] = map[string]any{
								"200": map[string]any{
									"description": "OK",
									"content": map[string]any{
										"application/json": map[string]any{
											"schema": oSchema,
										},
									},
								},
							}
						}
					default:
						oSchema = map[string]any{
							"type": "string",
						}
						body["responses"] = map[string]any{
							"200": map[string]any{
								"description": "OK",
								"content": map[string]any{
									"text/plain": map[string]any{
										"schema": oSchema,
									},
								},
							},
						}
					}
				}

				if !option.SkipWrapperCheck {
					menu = w.Menu()
					if menu != nil && menu.isDataWrapper() && oT != nil {
						var (
							dummyIn  = reflect.New(oT).Interface()
							dummyErr = errors.New("I am a dummy error")
							wV       reflect.Value
							wVT      reflect.Type
						)
						okWrapped, _ := any(menu.(iMenu[D]).Dependency()).(IWebCookwareWithDataWrapper).WrapWebOutput(dummyIn, nil)
						if okWrapped != nil {
							wV = reflect.ValueOf(okWrapped)
							wVT = wV.Type()
							if wVT.Kind() == reflect.Ptr {
								wV = wV.Elem()
								wVT = wVT.Elem()
							}
							if wVT.Kind() == reflect.Struct {
								oSchema = swaggerParseWrapper(wVT, wV, dummyIn, dummyErr, oSchema)
								body["responses"] = map[string]any{
									"200": map[string]any{
										"description": "OK",
										"content": map[string]any{
											"application/json": map[string]any{
												"schema": oSchema,
											},
										},
									},
								}
							}
						}
					}
				}

			} else {
				body["responses"] = map[string]any{
					"200": map[string]any{
						"description": "OK",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type": "object",
								},
							},
						},
					},
				}
			}
			{

				var (
					ref                    string
					urlParams, queryParams []paramType
					queryParamsRequired    []string
				)
				if iS, ok := i.(ISwaggerType); ok {
					ref, urlParams = iS.ToSwaggerType(i, types)
					body["requestBody"] = map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": ref,
								},
							},
						},
						"required": true,
					}
				} else if swaggerType, ok := swaggerTypes[iT]; ok {
					types[swaggerType.Name] = swaggerType.Body
					body["requestBody"] = map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"$ref": "#/components/schemas/" + swaggerType.Name,
								},
							},
						},
						"required": true,
					}
					urlParams = swaggerType.UrlParams
				} else {
					switch i.(type) {
					case string:
						/*parameters = append(parameters, map[string]any{
							"name":        "p1",
							"in":          "path",
							"required":    true,
							"description": "",
							"schema": map[string]any{
								"type": "string",
							},
						})*/

					default:
						if iT != nil {
							if iT.Kind() == reflect.Ptr {
								iT = iT.Elem()
							}
							if iT.Kind() == reflect.Struct {
								subTypeKey, urlParams, queryParams, queryParamsRequired = swaggerParseStruct(iT, types)
								if subTypeKey != "" {
									body["requestBody"] = map[string]any{
										"content": map[string]any{
											"application/json": map[string]any{
												"schema": map[string]any{
													"$ref": "#/components/schemas/" + subTypeKey,
												},
											},
										},
										"required": true,
									}
								}
							}
						}

					}
				}

				if len(urlParams) != 0 {
					if patternUrlParams := findUrlParamPatterns(url); len(patternUrlParams) > 0 {
						patternUrlParamsTypes := make([]paramType, 0)
						for _, pup := range patternUrlParams {
							patternUrlParamsTypes = append(patternUrlParamsTypes, paramType{Name: pup})
						}
						urlParams = combineAndRemoveDuplicates(patternUrlParamsTypes, urlParams)
					}
					var (
						params = make([]map[string]any, len(urlParams))
					)
					for i, p := range urlParams {
						params[i] = map[string]any{
							"name":        p,
							"in":          "path",
							"required":    true,
							"description": "",
							"schema": map[string]any{
								"type": "string",
							},
						}
					}
					body["parameters"] = params
				}
				if len(queryParams) != 0 {
					var (
						params = make([]map[string]any, len(queryParams))
					)
					for _, p := range queryParams {
						params = append(params, map[string]any{
							"name":        p,
							"in":          "query",
							"required":    ContainsValueInSlice(queryParamsRequired, p.Name),
							"description": "",
							"schema": map[string]any{
								"type": "string",
							},
						})

					}
					body["parameters"] = params
				}
			}

			if len(action.Security()) > 0 {
				security := make([]map[string][]string, 0)
				for _, s := range action.Security() {
					security = append(security, map[string][]string{s: make([]string, 0)})
				}
				body["security"] = security
			} else if option.SecurityMethod != "" {
				body["security"] = []map[string][]string{
					{
						option.SecurityMethod: make([]string, 0),
					},
				}
			}
			body["description"] = action.Desc()
			body["operationId"] = action.OperationId()
			body["summary"] = action.Summary()
			body["tags"] = action.Tags()

			m := map[string]any{
				strings.ToLower(method): body,
			}
			if patternUrlParams := findUrlParamPatterns(url); len(patternUrlParams) > 0 {
				patternUrlParamsTypes := make([]paramType, 0)
				for _, pup := range patternUrlParams {
					patternUrlParamsTypes = append(patternUrlParamsTypes, paramType{Name: pup})
				}
				urlParams = combineAndRemoveDuplicates(patternUrlParamsTypes, urlParams)
			}
			for _, p := range urlParams {
				parameters = append(parameters, map[string]any{
					// "name":        p[1 : len(p)-1],
					"name":        p.Name[1 : len(p.Name)-1],
					"in":          "path",
					"required":    true,
					"description": p.Desc,
					"schema": map[string]any{
						"type": "string",
					},
				})
			}
			for _, p := range queryParams {
				parameters = append(parameters, map[string]any{
					"name":        p.Name,
					"in":          "query",
					"required":    ContainsValueInSlice(queryParamsRequired, p.Name),
					"description": p.Desc,
					"schema": map[string]any{
						"type": "string",
					},
				})

			}
			body["parameters"] = parameters
			/*if url == "" {
				url = "/" + strings.Join(append(prefix, urlParams...), "/")
			} else {
				url = "/" + strings.Join(append(append(prefix, url), urlParams...), "/")
			}*/
			if v, ok := api[url]; ok {
				v.(map[string]any)[strings.ToLower(method)] = m[strings.ToLower(method)]
			} else {
				api[url] = m
			}

		} else {

			fmt.Printf("unknow type node : %T\n", node)
		}
	}
	return
}

func swaggerParseWrapper(vT reflect.Type, v reflect.Value, in any, err error, oSchema map[string]any) map[string]any {
	var (
		fT reflect.StructField
	)
	if !v.IsValid() || !v.IsValid() {
		v = reflect.New(vT).Elem()
	}
	if vT.Kind() != reflect.Struct {
		return map[string]any{
			"type": swaggerParseType(vT),
		}
	}
	res := map[string]any{
		"properties": map[string]any{},
		"type":       "object",
	}
	for i, l := 0, vT.NumField(); i < l; i++ {
		fT = vT.Field(i)
		if fT.Anonymous || !fT.IsExported() {
			continue
		}
		switch fT.Type.Kind() {
		case reflect.Interface:
			if v.Field(i).Interface() == in {
				res["properties"].(map[string]any)[fT.Name] = oSchema
			} else {
				res["properties"].(map[string]any)[fT.Name] = swaggerParseWrapper(fT.Type, v.Field(i), in, err, oSchema)
			}
		case reflect.Ptr:
			if v.Field(i).Interface() == in {
				res["properties"].(map[string]any)[fT.Name] = oSchema
			} else {
				res["properties"].(map[string]any)[fT.Name] = swaggerParseWrapper(fT.Type.Elem(), v.Field(i).Elem(), in, err, oSchema)
			}
		case reflect.Struct:
			res["properties"].(map[string]any)[fT.Name] = swaggerParseWrapper(fT.Type, v.Field(i), in, err, oSchema)
		//case reflect.Slice, reflect.Array:
		//
		//	pp := map[string]any{}
		//	res["properties"].(map[string]any)[fT.Name] = pp
		//	pp["type"] = "array"
		//	if fT.Type.Elem().Kind() == reflect.Ptr {
		//		fT.Type = fT.Type.Elem()
		//	}
		//	if eT, ok := reflect.New(fT.Type.Elem()).Elem().Interface().(hasTypeForExport); ok {
		//		fT.Type = reflect.SliceOf(reflect.TypeOf(eT.TypeForExport()))
		//	}
		//	if fT.Type.Elem().Kind() == reflect.Struct {
		//		switch fT.Type.Elem() {
		//		case timeType:
		//			pp["items"] = map[string]any{
		//				"type":   "string",
		//				"format": "date-time",
		//			}
		//		default:
		//			sub := swaggerParseWrapper(fT.Type.Elem(), reflect.New(fT.Type.Elem()).Elem(), in, err, oSchema)
		//			pp["items"] = sub
		//		}
		//	} else {
		//		pp["items"] = map[string]any{
		//			"type": swaggerParseType(fT.Type.Elem()),
		//		}
		//	}
		case reflect.Map:
			pp := map[string]any{}
			res["properties"].(map[string]any)[fT.Name] = pp
			pp["type"] = "object"
			var (
				mr = v.Field(i).MapRange()
			)
			if v.Field(i).Len() == 0 {
				if eT, ok := reflect.New(fT.Type.Elem()).Elem().Interface().(hasTypeForExport); ok {
					fT.Type = reflect.MapOf(fT.Type.Key(), reflect.TypeOf(eT.TypeForExport()))
				}
				if fT.Type.Elem().Kind() == reflect.Struct {
					switch fT.Type.Elem() {
					case timeType:
						pp["additionalProperties"] = map[string]any{
							"type":   "string",
							"format": "date-time",
						}
					default:
						sub := swaggerParseWrapper(fT.Type.Elem(), reflect.New(fT.Type.Elem()).Elem(), in, err, oSchema)
						pp["additionalProperties"] = sub
					}
				} else {
					pp["additionalProperties"] = map[string]any{
						"type": swaggerParseType(fT.Type.Elem()),
					}
				}
			} else {
				for mr.Next() {
					if mr.Value().Interface() == in {
						res["properties"].(map[string]any)[mr.Key().String()] = oSchema
					} else {
						res["properties"].(map[string]any)[mr.Key().String()] = swaggerParseWrapper(fT.Type.Elem(), mr.Value(), in, err, oSchema)
					}
				}
			}
		default:
			res["properties"].(map[string]any)[fT.Name] = map[string]any{
				"type": swaggerParseType(fT.Type),
			}
		}
		if mandate := fT.Tag.Get("required"); mandate != "" {
			mandateValue, err := strconv.ParseBool(mandate)
			if err != nil {
				mandateValue = false
			}
			if mandateValue {
				if res["required"] == nil {
					res["required"] = []string{fT.Name}
				} else {
					res["required"] = append(res["required"].([]string), fT.Name)
				}
			}
		}
	}
	return res
}

var (
	timeType      = reflect.TypeOf(time.Time{})
	genericPathRx = regexp.MustCompile(`[\[\,]\*?(?:\[[0-9]*\])?\*?([^,\[\]]+\.)[a-zA-Z0-9_]+`)
)

func swaggerParseStruct(t reflect.Type, allDefs map[string]any) (typeKey string, urlParams, queryParams []paramType, queryParamsRequired []string) {
	var (
		properties map[string]any
		required   = []string{}
		def        = map[string]any{"type": "object"}
	)
	typeKey = t.Name()
	if matches := genericPathRx.FindAllStringSubmatch(typeKey, -1); matches != nil {
		for _, match := range matches {
			typeKey = strings.Replace(typeKey, match[1], "", 1)
		}
		typeKey = strcase.ToLowerCamel(strings.Replace(strings.Replace(typeKey, ",", " ", -1), "[", " ", -1))
	} else {
		typeKey = strcase.ToLowerCamel(strings.Replace(typeKey, ".", " ", -1))
	}
	parts := strings.Split(typeKey, ".")
	if len(parts) > 2 {
		typeKey = strcase.ToLowerCamel(parts[len(parts)-2] + " " + parts[len(parts)-1])
	}
	if _, ok := allDefs[typeKey]; ok {
		return
	}
	allDefs[typeKey] = def
	properties, urlParams, queryParams, required, queryParamsRequired = swaggerParseStructFields(t, allDefs)
	if len(properties) > 0 {
		def["properties"] = properties
		if len(required) > 0 {
			def["required"] = required
		}
	}
	if len(properties) == 0 && (len(urlParams) != 0 || len(queryParams) != 0) {
		delete(allDefs, typeKey)
		typeKey = ""
	}
	return
}

func swaggerParseStructFields(t reflect.Type, allDefs map[string]any) (properties map[string]any, urlParams, queryParams []paramType, required, queryParamsRequired []string) {

	properties = map[string]any{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Tag.Get("json") == "-" {
			continue
		}
		if f.Tag.Get("urlParam") != "" {
			urlParams = append(urlParams, paramType{Name: f.Tag.Get("urlParam"), Desc: f.Tag.Get("desc")})
			continue
		}
		if f.Tag.Get("queryParam") != "" {
			queryParams = append(queryParams, paramType{Name: f.Tag.Get("queryParam"), Desc: f.Tag.Get("desc")})
			if mandate := f.Tag.Get("required"); mandate != "" {
				mandateValue, err := strconv.ParseBool(mandate)
				if err != nil {
					mandateValue = false
				}
				if mandateValue {
					queryParamsRequired = append(queryParamsRequired, f.Name)
				}
			}
			continue
		}
		fType := f.Type
		if eT, ok := reflect.New(fType).Elem().Interface().(hasTypeForExport); ok {
			fType = reflect.TypeOf(eT.TypeForExport())
		}
		if !f.IsExported() {
			continue
		}
		pp := map[string]any{}
		if desc := f.Tag.Get("desc"); desc != "" {
			pp["description"] = desc
		}
		if mandate := f.Tag.Get("required"); mandate != "" {
			mandateValue, err := strconv.ParseBool(mandate)
			if err != nil {
				mandateValue = false
			}
			if mandateValue {
				required = append(required, f.Name)
			}
		}
		if fType == nil {
			pp["type"] = "object"
		} else {
			var (
				fieldInstance = reflect.New(fType).Elem().Interface()
				subTypeKey    string
			)
			if fS, ok := fieldInstance.(ISwaggerType); ok {
				pp["$ref"], _ = fS.ToSwaggerType(fieldInstance, allDefs)
			} else if swaggerType, ok := swaggerTypes[fType]; ok {
				pp["$ref"] = "#/components/schemas/" + swaggerType.Name
				allDefs[swaggerType.Name] = swaggerType.Body
				urlParams = append(urlParams, swaggerType.UrlParams...)
			} else {

				if fType.Kind() == reflect.Ptr {
					fType = fType.Elem()
				}

				switch fType.Kind() {
				case reflect.Struct:
					switch fType {
					case timeType:
						pp["type"] = "string"
						pp["format"] = "date-time"
					default:
						if f.Anonymous {
							anonymouseProps, anonymouseUrlParams, anonymusQueryParams, anonymusRequired, anonymusQueryParamRequired := swaggerParseStructFields(fType, allDefs)
							for k, v := range anonymouseProps {
								properties[k] = v
							}
							urlParams = append(urlParams, anonymouseUrlParams...)
							queryParams = append(queryParams, anonymusQueryParams...)
							required = append(required, anonymusRequired...)
							queryParamsRequired = append(queryParamsRequired, anonymusQueryParamRequired...)
							continue
						} else {
							subTypeKey, _, _, _ = swaggerParseStruct(fType, allDefs)
							if subTypeKey != "" {
								pp["$ref"] = "#/components/schemas/" + subTypeKey
							}
						}
					}
				case reflect.Slice, reflect.Array:
					pp["type"] = "array"
					if fType.Elem().Kind() == reflect.Ptr {
						fType = fType.Elem()
					}
					if eT, ok := reflect.New(fType.Elem()).Elem().Interface().(hasTypeForExport); ok {
						fType = reflect.SliceOf(reflect.TypeOf(eT.TypeForExport()))
						//fmt.Println(fType)
					}
					if fType.Elem().Kind() == reflect.Struct {
						var (
							fieldInstance = reflect.New(fType.Elem()).Elem().Interface()
						)
						if fS, ok := fieldInstance.(ISwaggerType); ok {
							ref, _ := fS.ToSwaggerType(fieldInstance, allDefs)
							pp["items"] = map[string]any{
								"$ref": ref,
							}
						} else {
							switch fType.Elem() {
							case timeType:
								pp["items"] = map[string]any{
									"type":   "string",
									"format": "date-time",
								}
							default:
								subTypeKey, _, _, _ = swaggerParseStruct(fType.Elem(), allDefs)
								if subTypeKey != "" {
									pp["items"] = map[string]any{
										"$ref": "#/components/schemas/" + subTypeKey,
									}
								}
							}
						}
					} else {
						pp["items"] = map[string]any{
							"type": swaggerParseType(fType.Elem()),
						}
					}
				case reflect.Map:
					if fType.Elem().Kind() == reflect.Ptr {
						fType = fType.Elem()
					}
					if eT, ok := reflect.New(fType.Elem()).Elem().Interface().(hasTypeForExport); ok {
						fType = reflect.MapOf(fType.Key(), reflect.TypeOf(eT.TypeForExport()))
					}
					if fType.Elem().Kind() == reflect.Struct {
						pp["type"] = "object"
						switch fType.Elem() {
						case timeType:
							pp["additionalProperties"] = map[string]any{
								"type":   "string",
								"format": "date-time",
							}
						default:
							subTypeKey, _, _, _ = swaggerParseStruct(fType.Elem(), allDefs)
							if subTypeKey != "" {
								pp["additionalProperties"] = map[string]any{
									"$ref": "#/components/schemas/" + subTypeKey,
								}
							}
						}
					} else {
						pp["additionalProperties"] = map[string]any{
							"type": swaggerParseType(fType.Elem()),
						}
					}
				default:
					pp["type"] = swaggerParseType(fType)
				}
			}
		}
		properties[f.Name] = pp

	}
	return
}

func swaggerParseType(t reflect.Type, isMap ...bool) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	default:
		if t.Name() == "" || (len(isMap) > 0 && isMap[0]) {
			return "object"
		}
		return t.Name()
	}
}
