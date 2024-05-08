package kitchen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"cd.codes/galaxy/kitchen/stringMap"
	"github.com/gorilla/mux"
	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/proto"
)

type PipelineActionInput[M IPipelineModel, I any] struct {
	Input  I
	Model  M
	Before map[string]string
	Status PipelineStatus
}

type PipelineActionOutput[M IPipelineModel, O any] struct {
	Output O
	Model  M
}

type PipelineAction[D IPipelineCookware[M], M IPipelineModel, I any, O any] struct {
	Dish[D, *PipelineActionInput[M, I], *PipelineActionOutput[M, O]]
	stage           iPipelineStage[D, M]
	nextStatus      PipelineStatus
	willCreateModel bool
	canMap          bool
	urlPath         string
	newInput        func() I
}

func initPipelineAction[D IPipelineCookware[M], M IPipelineModel](parent iCookbook[D], act iPipelineAction[D, M], name string, tags reflect.StructTag) {
	act.initAction(parent, act, name, tags)
}

// set next status
func (p *PipelineAction[D, M, I, O]) SetNextStage(stage IPipelineStage) *PipelineAction[D, M, I, O] {
	p.nextStatus = stage.Status()
	return p
}
func (p PipelineAction[D, M, I, O]) Status() PipelineStatus {
	if p.stage == nil {
		return ""
	}
	return p.stage.Status()
}

func (p *PipelineAction[D, M, I, O]) SetCooker(cooker PipelineDishCooker[D, M, I, O]) *PipelineAction[D, M, I, O] {
	var (
		m M
	)
	_, p.canMap = any(m).(IPipelineModelCanMap)
	p.cooker = func(i IContext[D], p *PipelineActionInput[M, I]) (output *PipelineActionOutput[M, O], err error) {
		outputWrap := &PipelineActionOutput[M, O]{}
		outputWrap.Output, outputWrap.Model, err = cooker(i.(IPipelineContext[D, M]), p.Model, p.Input)
		return outputWrap, err
	}
	return p
}

func (p *PipelineAction[D, M, I, O]) WillCreateModel() *PipelineAction[D, M, I, O] {
	p.willCreateModel = true
	return p
}

func (p *PipelineAction[D, M, I, O]) initAction(parent iCookbook[D], act iPipelineAction[D, M], name string, fieldTag reflect.StructTag) {
	p.init(parent, act, name, fieldTag)
	p.stage, _ = any(parent).(iPipelineStage[D, M])
	p.urlPath = strcase.ToSnake(p.name)
	var (
		i     I
		iType = reflect.TypeOf(i)
	)
	if iType != nil && iType.Kind() == reflect.Ptr {
		p.newInput = func() I {
			return reflect.New(iType.Elem()).Interface().(I)
		}
	} else {
		p.newInput = func() I {
			var i I
			return i
		}
	}
}

var ErrInvalidStatus = errors.New("invalid pipeline status")

func (p PipelineAction[D, M, I, O]) ExecWithModel(ctx context.Context, model M, input I) (output O, err error) {
	return p.execWithModel(p.newCtx(ctx), model, input)
}
func (p PipelineAction[D, M, I, O]) ExecWithModelAndDep(ctx context.Context, dep D, model M, input I) (output O, err error) {
	return p.execWithModel(p.newCtx(ctx, dep), model, input)
}

func (p PipelineAction[D, M, I, O]) execWithModel(ctx *Context[D], model M, input I) (output O, err error) {
	var (
		tx         IDbTx
		dep        = ctx.Cookware()
		oStatus    PipelineStatus
		modelIsNil = reflect.ValueOf(model).IsNil()
		inputWrap  = &PipelineActionInput[M, I]{Input: input, Model: model}
	)
	if !modelIsNil {
		oStatus = model.GetStatus()
		inputWrap.Before = p.ModelToMap(model)
		inputWrap.Status = oStatus
	}
	if p.stage != nil && oStatus != p.stage.Status() {
		return output, ErrInvalidStatus
	}
	tx, err = dep.BeginTx(ctx)
	if err != nil {
		return output, err
	}
	ctx.dish = &p
	var outputWrap *PipelineActionOutput[M, O]
	outputWrap, err = p.Dish.cook(&PipelineContext[D, M]{
		Context: *ctx,
		tx:      tx,
	}, inputWrap, func(output *PipelineActionOutput[M, O], resErr error) error {
		if resErr == nil {
			if p.nextStatus != "" {
				output.Model.SetStatus(p.nextStatus)
			}
			resErr = dep.SaveModel(tx, output.Model, oStatus)
		}
		return resErr
	})
	output = outputWrap.Output
	err = dep.FinishTx(tx, err)
	return
}
func (p PipelineAction[D, M, I, O]) ModelToMap(model IPipelineModel) map[string]string {
	if p.canMap {
		return model.(IPipelineModelCanMap).ToMap()
	}
	return stringMap.FromStruct(model)
}

func (p PipelineAction[D, M, I, O]) ExecByIdAndDep(ctx context.Context, dep D, input I, modelId ...any) (output O, err error) {
	var (
		model M
	)
	if len(modelId) != 0 {
		model, err = dep.GetModelById(ctx, modelId...)
		if err != nil {
			return output, err
		}
	} else if !p.willCreateModel {
		return output, errors.New("pk is empty")
	}
	return p.ExecWithModelAndDep(ctx, dep, model, input)
}

func (p PipelineAction[D, M, I, O]) execById(ctx *Context[D], input I, modelId ...any) (output O, err error) {
	var (
		model M
	)
	if len(modelId) != 0 {
		model, err = ctx.Cookware().GetModelById(ctx, modelId...)
		if err != nil {
			return output, err
		}
	} else if !p.willCreateModel {
		return output, errors.New("pk is empty")
	}
	return p.execWithModel(ctx, model, input)
}

func (p PipelineAction[D, M, I, O]) ExecById(ctx context.Context, input I, modelId ...any) (output O, err error) {
	return p.execById(p.newCtx(ctx), input, modelId...)
}

func (a PipelineAction[D, M, I, O]) ServeHttp() (method string, urlParts []string, handler http.HandlerFunc) {
	url, urlParams, _, method, _ := a.urlAndMethod()
	urlParamsN := make([]string, 0)
	for _, up := range urlParams {
		urlParamsN = append(urlParamsN, up.Name)
	}
	if url != "" {
		urlParts = append([]string{url}, urlParamsN...)
	} else {
		urlParts = urlParamsN
	}
	return method, urlParts, func(w http.ResponseWriter, r *http.Request) {
		input, raw, err := a.parseRequestToInput(r)
		if err == nil {
			ww := &webWriter{ResponseWriter: w}
			ctx, err := a.newWebCtx(ww, r)
			if err != nil {
				a.webReturn(w, nil, err)
				return
			}
			ctx.WebBundle = &webBundle{RequestBody: raw, Request: r, Response: ww}
			if a.willCreateModel {
				var m M
				output, err := a.execWithModel(ctx, m, input)
				a.webReturn(w, output, err)
				return
			} else {
				urlParts := strings.Split(r.URL.Path, "/")
				if id := urlParts[len(urlParts)-1]; id != a.urlPath {
					output, err := a.execById(ctx, input, id)
					a.webReturn(w, output, err)
					return
				} else if input, ok := any(input).(PipelineActionInput[M, I]); ok {
					output, err := a.execWithModel(ctx, input.Model, input.Input)
					a.webReturn(w, output, err)
					return
				}
			}
		}
		a.webReturn(w, nil, WebErr{Err: err, HttpStatus: http.StatusBadRequest, ErrorCode: http.StatusBadRequest})
		return
	}
}

func (a *PipelineAction[D, M, I, O]) urlAndMethod() (url string, urlParams, queryParams []paramType, method string, queryParamsRequired []string) {
	var (
		input I
		param string
	)
	if !a.willCreateModel {
		urlParams = append(urlParams, paramType{Name: "{id}"})
	}
	iType := reflect.TypeOf(input)
	if iType != nil {
		if iType.Kind() == reflect.Ptr {
			iType = iType.Elem()
		}
		if iType.Kind() == reflect.Struct {
			for i := 0; i < iType.NumField(); i++ {
				if param = iType.Field(i).Tag.Get("urlParam"); param != "" {
					urlParams = append(urlParams, paramType{Name: fmt.Sprintf("{%s}", param), Desc: iType.Field(i).Tag.Get("desc")})
					a.webUrlParamMap = append(a.webUrlParamMap, i)
				} else if param = iType.Field(i).Tag.Get("queryParam"); param != "" {
					queryParams = append(queryParams, paramType{Name: param, Desc: iType.Field(i).Tag.Get("desc")})
					a.webQueryParamMap = append(a.webQueryParamMap, nameAndPos{pos: i, name: param})
					if mandate := iType.Field(i).Tag.Get("required"); mandate != "" {
						mandateValue, err := strconv.ParseBool(mandate)
						if err != nil {
							mandateValue = false
						}
						if mandateValue {
							queryParamsRequired = append(queryParamsRequired, iType.Field(i).Name)
						}
					}
				}
			}
		}
	}
	switch a.Name() {
	case "GET", "HEAD", "OPTIONS", "PATCH", "TRACE", "CONNECT", "POST", "UPDATE", "DELETE", "PUT":
		method = a.Name()
	default:
		method = "POST"
		url = a.urlPath
	}
	return url, urlParams, queryParams, method, queryParamsRequired
}

func (a PipelineAction[D, M, I, O]) parseRequestToInput(r *http.Request) (input I, raw []byte, err error) {
	input = a.newInput()
	if a.isWebInput {
		raw, err = any(input).(IWebParsableInput).ParseRequestToInput(r)
	} else if r.Method == http.MethodGet || r.Method == http.MethodDelete {
		vals := mux.Vars(r)
		for k, v := range r.URL.Query() {
			vals[k] = v[0]
		}
		if len(vals) != 0 {
			switch any(input).(type) {
			case string:
				if v, ok := vals["p1"]; ok {
					input = any(v).(I)
					break
				}
				for _, v := range vals {
					input = any(v).(I)
					break
				}
			default:
				raw, _ = json.Marshal(vals)
				err = json.Unmarshal(raw, input)
			}
		}
	} else if raw, err = io.ReadAll(r.Body); err == nil && len(raw) != 0 {
		if msg, ok := any(input).(proto.Message); ok {
			err = proto.Unmarshal(raw, msg)
		} else {
			err = json.Unmarshal(raw, &input)
		}
	}
	return
}
func (p PipelineAction[D, M, I, O]) ToSwaggerRoute(types map[string]any, opt SwaggerOption) map[string]any {
	var (
		i                      I
		iType                  = reflect.TypeOf(i)
		o                      O
		oType                  = reflect.TypeOf(o)
		iParams                = make([]map[string]any, 0)
		iTypeName              string
		oParams                = map[string]any{}
		urlParams, queryParams []paramType
		queryParamsRequired    []string
		typeKey                string
		subTypeKey             string
	)
	if iS, ok := any(i).(ISwaggerType); ok {
		iTypeName, urlParams = iS.ToSwaggerType(i, types)
	} else {
		if iType.Kind() == reflect.Ptr {
			iType = iType.Elem()
		}
		typeKey, urlParams, queryParams, queryParamsRequired = swaggerParseStruct(iType, types)
		iTypeName = "#/components/schemas/" + typeKey
	}
	if oS, ok := any(o).(ISwaggerType); ok {
		oParams["$ref"], _ = oS.ToSwaggerType(o, types)
	} else if oType != nil {
		if oType.Kind() == reflect.Ptr {
			oType = oType.Elem()
		}
		subTypeKey, _, _, _ = swaggerParseStruct(oType, types)
		oParams["$ref"] = "#/components/schemas/" + subTypeKey
	} else {
		oParams["type"] = "string"
	}

	for _, v := range urlParams {
		iParams = append(iParams, map[string]any{
			"name":        v.Name,
			"in":          "path",
			"required":    true,
			"description": v.Desc,
			"schema": map[string]any{
				"type": "string",
			},
		})
	}
	for _, v := range queryParams {
		iParams = append(iParams, map[string]any{
			"name":        v,
			"in":          "query",
			"required":    ContainsValueInSlice(queryParamsRequired, v.Name),
			"description": "",
			"schema": map[string]any{
				"type": "string",
			},
		})
	}
	url, urlParams, queryParams, method, queryParamsRequired := p.urlAndMethod()
	urlParamsN := make([]string, 0)
	for _, up := range urlParams {
		urlParamsN = append(urlParamsN, up.Name)
	}
	if url == "" {
		url = strings.Join(urlParamsN, "/")
	} else {
		url = strings.Join(append([]string{url}, urlParamsN...), "/")
	}
	for _, p := range urlParams {
		iParams = append(iParams, map[string]any{
			"name":        p.Name,
			"in":          "path",
			"required":    true,
			"description": p.Desc,
			"schema": map[string]any{
				"type": "string",
			},
		})
	}
	for _, p := range queryParams {
		iParams = append(iParams, map[string]any{
			"name":        p.Name,
			"in":          "query",
			"required":    ContainsValueInSlice(queryParamsRequired, p.Name),
			"description": p.Desc,
			"schema": map[string]any{
				"type": "string",
			},
		})
	}
	body := map[string]any{
		url: map[string]any{
			strings.ToLower(method): map[string]any{
				"description": p.Desc(),
				"parameters":  iParams,
				"requestBody": map[string]any{
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": iTypeName,
							},
						},
					},
					"required": true,
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "OK",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": oParams,
							},
						},
					},
				},
			},
		},
	}
	if opt.SecurityMethod != "" {
		body[url].(map[string]any)[strings.ToLower(method)].(map[string]any)["security"] = []map[string][]string{
			{
				opt.SecurityMethod: make([]string, 0),
			},
		}
	}
	return body
}

func PipelineActionInputToAny[M IPipelineModel](input any) PipelineActionInput[M, any] {
	var (
		iV = reflect.ValueOf(input)
		mV reflect.Value
	)
	if iV.Type().Kind() == reflect.Ptr {
		iV = iV.Elem()
	}
	mV = iV.Field(1)
	return PipelineActionInput[M, any]{
		Input:  iV.Field(0).Interface(),
		Model:  mV.Interface().(M),
		Before: iV.Field(2).Interface().(map[string]string),
	}
}

func PipelineActionOutputToAny[M IPipelineModel](output any) PipelineActionOutput[M, any] {
	var (
		iV = reflect.ValueOf(output)
		mV reflect.Value
	)
	if iV.Type().Kind() == reflect.Ptr {
		iV = iV.Elem()
	}
	mV = iV.Field(1)
	return PipelineActionOutput[M, any]{
		Output: iV.Field(0).Interface(),
		Model:  mV.Interface().(M),
	}
}
