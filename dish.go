package kitchen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/proto"
)

type nameAndPos struct {
	name string
	pos  int
}
type Dish[D ICookware, I any, O any] struct {
	cookbook[D, I, O]
	sets             []iSet[D]
	menu             iMenu[D]
	name             string
	cooker           DishCooker[D, I, O]
	panicRecover     bool
	newInput         func() I
	id               uint32
	asyncChan        chan asyncTask[D, I, O]
	isWebDep         bool
	isWebInput       bool
	webUrlParamMap   []int
	webQueryParamMap []nameAndPos
	path             *string
	desc             string
	operationId      string
	summary          string
	tags             []string
	method           string
	security         []string
}

type asyncTask[D ICookware, I, O any] struct {
	ctx      IContext[D]
	input    I
	callback func(O, error)
}

func initDish[D ICookware](parent iCookbook[D], action iDish[D], name string, tags reflect.StructTag) {
	action.init(parent, action, name, tags)
}

func (a *Dish[D, I, O]) extractTags(tags reflect.StructTag, key string, fn func(string) *Dish[D, I, O]) {
	fn(tags.Get(key))
}

func (a *Dish[D, I, O]) init(parent iCookbook[D], action iDish[D], name string, tags reflect.StructTag) {
	a.name = name
	if val, ok := tags.Lookup("path"); ok {
		a.name = val
	}
	if val, ok := tags.Lookup("urlParam"); ok {
		a.name = fmt.Sprintf("{%s}", val)
	}
	switch name {
	case "GET", "HEAD", "OPTIONS", "PATCH", "TRACE", "CONNECT", "POST", "UPDATE", "DELETE", "PUT":
		a.method = name
		if a.name == name {
			a.name = ""
		}
	}
	a.instance = action
	a.extractTags(tags, "desc", a.SetDesc)
	a.extractTags(tags, "operationId", a.SetOperationId)
	a.extractTags(tags, "summary", a.SetSummary)
	a.extractTags(tags, "tags", a.SetTags)
	a.extractTags(tags, "method", a.SetHttpMethod)
	if set, ok := any(parent).(iSet[D]); ok {
		var setNames []string
		a.sets = set.Tree()
		for _, g := range a.sets {
			a.inherit(g)
			setNames = append([]string{g.Name()}, setNames...)
		}
		a.inherit(set.menu())
		a.menu = set.menu()
		if len(setNames) != 0 {
			a.fullName = fmt.Sprintf("%s.%s.%s", a.menu.Name(), strings.Join(setNames, "."), a.name)
		} else {
			a.fullName = fmt.Sprintf("%s.%s", a.menu.Name(), a.name)
		}
	} else {
		a.inherit(parent)
		a.menu = any(parent).(iMenu[D])
		a.fullName = fmt.Sprintf("%s.%s", a.menu.Name(), a.name)
	}
	_, a.isWebDep = any(a.menu.Cookware()).(IWebCookware)
	var input I
	_, a.isWebInput = any(input).(IWebParsableInput)
	a.id = uint32(a.menu.pushDish(action))
	a.isTraceable = a.menu.isTraceableDep()
	var (
		iType = reflect.TypeOf((*I)(nil)).Elem()
	)
	if iType.Kind() == reflect.Ptr {
		a.newInput = func() I {
			return reflect.New(iType.Elem()).Interface().(I)
		}
	} else {
		a.newInput = func() I {
			var i I
			return i
		}
	}
}

func (a *Dish[D, I, O]) OverridePath(path string) *Dish[D, I, O] {
	a.path = &path
	return a
}

func (a Dish[D, I, O]) Id() uint32 {
	return a.id
}

func (a Dish[D, I, O]) IO() (any, any) {
	var (
		i I
		o O
	)
	return i, o
}

func (a Dish[D, I, O]) Name() string {
	if a.path != nil {
		return *a.path
	}
	return a.name
}

func (a Dish[D, I, O]) FullName() string {
	return a.fullName
}

func (a Dish[D, I, O]) Menu() IMenu {
	return a.menu
}

func (a Dish[D, I, O]) Desc() string {
	return a.desc
}

func (a Dish[D, I, O]) OperationId() string {
	return a.operationId
}

func (a Dish[D, I, O]) Summary() string {
	return a.summary
}

func (a Dish[D, I, O]) Tags() []string {
	return a.tags
}

func (a *Dish[D, I, O]) SetDesc(desc string) *Dish[D, I, O] {
	a.desc = desc
	return a
}

func (a *Dish[D, I, O]) SetOperationId(id string) *Dish[D, I, O] {
	a.operationId = id
	return a
}

func (a *Dish[D, I, O]) SetSummary(summary string) *Dish[D, I, O] {
	a.summary = summary
	return a
}

func (a *Dish[D, I, O]) SetSecurity(security []string) *Dish[D, I, O] {
	a.security = security
	return a
}

func (a Dish[D, I, O]) Security() []string {
	return a.security
}

func (a *Dish[D, I, O]) SetTags(tags string) *Dish[D, I, O] {
	vals := make([]string, 0)
	for _, v := range strings.Split(tags, ",") {
		val := strings.ToLower(strings.Trim(v, " "))
		if len(val) > 0 {
			vals = append(vals, val)
		}
	}
	if len(vals) > 0 {
		a.tags = vals
	} else {
		a.tags = []string{"default"}
	}
	return a
}

func (a *Dish[D, I, O]) SetHttpMethod(method string) *Dish[D, I, O] {
	if method != "" {
		a.method = method
	}
	return a
}

func (a Dish[D, I, O]) Sets() []ISet {
	var res = make([]ISet, len(a.sets))
	for i, s := range a.sets {
		res[i] = s
	}
	return res
}

func (a *Dish[D, I, O]) SetAsyncCooker(ctx context.Context, buffSize, threadSize int, cooker DishCooker[D, I, O]) {
	if a.asyncChan != nil {
		close(a.asyncChan)
		a.asyncChan = nil
	}
	if cooker != nil && threadSize > 0 && buffSize > 0 {
		a.asyncChan = make(chan asyncTask[D, I, O], buffSize)
		for i := 0; i < threadSize; i++ {
			go func() {
				var (
					err    error
					node   IDishServe
					t      asyncTask[D, I, O]
					ok     bool
					ch     = a.asyncChan
					output O
				)
				if a.panicRecover {
					defer func() {
						if rec := recover(); rec != nil {
							if a.isTraceable && t.ctx != nil && t.ctx.TraceSpan() != nil {
								t.ctx.TraceSpan().AddEvent("panic", map[string]any{"panic": rec, "stack": string(debug.Stack())})
							} else {
								fmt.Printf("panicRecover from panic: \n%v\n%s", rec, string(debug.Stack()))
							}
						}
					}()
				}
				for {
					select {
					case <-ctx.Done():
						close(ch)
						a.asyncChan = nil
					case t, ok = <-ch:
						if !ok {
							return
						}
						node = a.start(t.ctx, t.input, false)
						output, err = cooker(t.ctx, t.input)
						a.emitAfterCook(t.ctx, t.input, output, err)
						node.finish(nil, err)
						if t.callback != nil {
							t.callback(output, err)
						}
					}
				}
			}()
		}
	}
}
func (a *Dish[D, I, O]) SetAsyncExecer(ctx context.Context, buffSize, threadSize int, cooker DishCooker[D, I, O]) {
	a.SetAsyncCooker(ctx, buffSize, threadSize, cooker)
}
func (a *Dish[D, I, O]) SetExecer(cooker DishCooker[D, I, O]) *Dish[D, I, O] {
	return a.SetCooker(cooker)
}

func (a *Dish[D, I, O]) SetCooker(cooker DishCooker[D, I, O]) *Dish[D, I, O] {
	if a.asyncChan != nil {
		close(a.asyncChan)
		a.asyncChan = nil
	}
	a.cooker = cooker
	return a
}

func (a *Dish[D, I, O]) PanicRecover(recover bool) *Dish[D, I, O] {
	a.panicRecover = recover
	return a
}

func (a *Dish[D, I, O]) Cookware() D {
	d := a.menu.Cookware()
	return d
}

func (a *Dish[D, I, O]) Dependency() D {
	d := a.menu.Cookware()
	return d
}

var ErrCookerNotSet = errors.New("cooker not set")

func (a *Dish[D, I, O]) Cook(ctx context.Context, input I) (output O, err error) {
	if a.asyncChan != nil {
		l := &sync.Mutex{}
		l.Lock()
		err = a.ExecAsync(ctx, input, func(o O, e error) {
			output = o
			err = e
			l.Unlock()
		})
		if err != nil {
			l.Unlock()
			return
		}
		l.Lock()
		return
	}
	return a.cook(a.newCtx(ctx), input, nil)
}

func (a *Dish[D, I, O]) ExecWithDep(ctx context.Context, dep D, input I) (output O, err error) {
	return a.CookWithCookware(ctx, dep, input)
}

func (a *Dish[D, I, O]) CookWithCookware(ctx context.Context, cookware D, input I) (output O, err error) {
	return a.Cook(a.newCtx(ctx, cookware), input)
}

func (a *Dish[D, I, O]) Exec(ctx context.Context, input I) (output O, err error) {
	return a.Cook(ctx, input)
}

func (a *Dish[D, I, O]) ExecAsync(ctx context.Context, input I, optionalCallback ...func(O, error)) error {
	return a.CookAsync(ctx, input, optionalCallback...)
}

func (a *Dish[D, I, O]) CookAsync(ctx context.Context, input I, optionalCallback ...func(O, error)) error {
	if a.asyncChan == nil {
		return errors.New("async cooker not set")
	}
	var cb func(O, error)
	if len(optionalCallback) != 0 {
		cb = optionalCallback[0]
	}
	a.asyncChan <- asyncTask[D, I, O]{ctx: a.newCtx(ctx), input: input, callback: cb}
	return nil
}

func (a *Dish[D, I, O]) cook(ctx IContext[D], input I, followUp func(O, error) error) (output O, err error) {
	node := a.start(ctx, input, a.panicRecover)
	if a.cooker == nil {
		err = ErrCookerNotSet
	} else {
		output, err = a.cooker(ctx, input)
		if followUp != nil {
			err = followUp(output, err)
		}
	}
	a.emitAfterCook(ctx, input, output, err)
	node.finish(output, err)
	return output, err
}

type webWriter struct {
	http.ResponseWriter
	returnedStatus bool
}

func (w *webWriter) WriteHeader(statusCode int) {
	if !w.returnedStatus {
		w.ResponseWriter.WriteHeader(statusCode)
		w.returnedStatus = true
	}
}

func (w *webWriter) Write(data []byte) (int, error) {
	return w.ResponseWriter.Write(data)
}

func (a *Dish[D, I, O]) ServeHttp() (method string, urlParts []string, handler http.HandlerFunc) {
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
		if err != nil {
			a.webReturn(w, nil, WebErr{Err: err, HttpStatus: http.StatusBadRequest, ErrorCode: http.StatusBadRequest})
			return
		}
		ww := &webWriter{ResponseWriter: w}
		ctx, err := a.newWebCtx(ww, r)
		if err != nil {
			a.webReturn(w, nil, err)
			return
		}
		ctx.WebBundle = &webBundle{RequestBody: raw, Request: r, Response: ww}
		output, err := a.cook(ctx, input, nil)

		a.webReturn(w, output, err)
	}
}

func (a *Dish[D, I, O]) urlAndMethod() (url string, urlParams, queryParams []paramType, method string, queryParamsRequired []string) {
	var (
		input I
		param string
	)
	method = a.method
	switch any(input).(type) {
	case string:
		urlParams = append(urlParams, paramType{Name: "{p1}"})
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
						urlParams = append(urlParams, paramType{Name: fmt.Sprintf("{%s}", param), Desc: iType.Field(i).Tag.Get("desc")})
						a.webUrlParamMap = append(a.webUrlParamMap, i)
					} else if param = iType.Field(i).Tag.Get("queryParam"); param != "" {
						queryParams = append(queryParams, paramType{Name: param, Desc: iType.Field(i).Tag.Get("desc")})
						if mandate := iType.Field(i).Tag.Get("required"); mandate != "" {
							mandateValue, err := strconv.ParseBool(mandate)
							if err != nil {
								mandateValue = false
							}
							if mandateValue {
								queryParamsRequired = append(queryParamsRequired, iType.Field(i).Name)
							}
						}
						a.webQueryParamMap = append(a.webQueryParamMap, nameAndPos{name: param, pos: i})
					}
				}
			}
		} else {
			method = setValueIfEmpty(method, "GET")
		}
	}
	//switch a.Name() {
	//case "GET", "HEAD", "OPTIONS", "PATCH", "TRACE", "CONNECT", "POST", "UPDATE", "DELETE", "PUT":
	//	method = a.Name()
	//default:
	url = strcase.ToSnake(a.Name())
	//}
	return url, urlParams, queryParams, method, queryParamsRequired
}

func (a Dish[D, I, O]) webReturn(w http.ResponseWriter, outputAny any, err error) {
	var httpCode int
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if a.Menu().isDataWrapper() {
			outputAny, httpCode = any(a.Dependency()).(IWebCookwareWithDataWrapper).WrapWebOutput(outputAny, err)
			w.WriteHeader(httpCode)
			data, err := json.Marshal(outputAny)
			if err != nil {
				fmt.Println("marshalling error:", err)
			}
			_, _ = w.Write(data)
		} else {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(err.Error()))
		}
		fmt.Println(err)
		return
	}
	if outputAny != nil {
		switch outputAny := outputAny.(type) {
		case string:
			w.Header().Set("Content-Type", "text/plain")
			if a.Menu().isDataWrapper() {
				output, httpCode := any(a.Dependency()).(IWebCookwareWithDataWrapper).WrapWebOutput(outputAny, err)
				data, err := json.Marshal(output)
				if err != nil {
					fmt.Println("marshalling error:", err)
				}
				w.WriteHeader(httpCode)
				_, _ = w.Write(data)
				return
			}
			_, _ = w.Write([]byte(outputAny))
		default:
			if msg, ok := outputAny.(proto.Message); ok {
				//todo wrap
				w.Header().Set("Content-Type", "application/octet-stream")
				data, _ := proto.Marshal(msg)
				_, _ = w.Write(data)
			} else {
				w.Header().Set("Content-Type", "application/json")
				if a.Menu().isDataWrapper() {
					outputAny, httpCode = any(a.Dependency()).(IWebCookwareWithDataWrapper).WrapWebOutput(outputAny, err)
					w.WriteHeader(httpCode)
				}
				data, err := json.Marshal(outputAny)
				if err != nil {
					fmt.Println("marshalling error:", err)
				}
				_, _ = w.Write(data)
			}
		}
	}
}

func (a Dish[D, I, O]) parseRequestToInput(r *http.Request) (input I, raw []byte, err error) {
	input = a.newInput()
	if a.isWebInput {
		raw, err = any(input).(IWebParsableInput).ParseRequestToInput(r)
	} else {
		var (
			urlParam map[string]string
			iVal     reflect.Value
			l        int
		)
		if r.Method == http.MethodGet || r.Method == http.MethodDelete {
			urlParam = mux.Vars(r)
			for k, v := range r.URL.Query() {
				urlParam[k] = v[0]
			}
			raw = []byte(r.URL.String())
			if len(urlParam) != 0 {
				switch any(input).(type) {
				case string:
					if v, ok := urlParam["p1"]; ok {
						input = any(v).(I)
						break
					}
					for _, v := range urlParam {
						input = any(v).(I)
						break
					}
					return
				default:
					raw, _ = json.Marshal(urlParam)
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
		if l = len(a.webUrlParamMap); l != 0 {
			var (
				params = strings.Split(r.URL.Path, "/")
			)
			params = params[len(params)-l:]
			iVal = reflect.ValueOf(input)
			if iVal.Kind() == reflect.Ptr {
				iVal = iVal.Elem()
			}
			for i, p := range a.webUrlParamMap {
				if p != -1 {
					iVal.Field(p).SetString(params[i])
				}
			}
		}
		if l = len(a.webQueryParamMap); l != 0 {
			var (
				q = r.URL.Query()
			)
			if !iVal.IsValid() {
				iVal = reflect.ValueOf(input)
				if iVal.Kind() == reflect.Ptr {
					iVal = iVal.Elem()
				}
			}
			for _, p := range a.webQueryParamMap {
				iVal.Field(p.pos).SetString(q.Get(p.name))
			}
		}
	}
	return
}

func (a *Dish[D, I, O]) newCtx(ctx context.Context, cookware ...D) *Context[D] {
	var (
		c  *Context[D]
		ok bool
	)
	if c, ok = ctx.(*Context[D]); ok {
		return c
	}
	c = &Context[D]{Context: ctx, menu: a.menu, sets: a.sets, dish: a}
	var (
		cw D
	)
	if len(cookware) != 0 {
		cw = cookware[0]
	} else {
		cw = a.Cookware()
	}
	if c.inherited, ok = ctx.(IContextWithSession); ok {
		if a.menu.isInheritableDep() {
			c.cookware = any(cw).(ICookwareInheritable).Inherit(c.inherited.RawCookware()).(D)
		} else {
			c.cookware = cw
		}
	} else {
		c.cookware = cw
	}
	if a.isTraceable {
		c.traceableDep = any(c.cookware).(ITraceableCookware)
	}
	return c
}

func (a *Dish[D, I, O]) newWebCtx(w http.ResponseWriter, r *http.Request) (*Context[D], error) {
	dep := a.Cookware()
	ctx := &Context[D]{Context: r.Context(), menu: a.menu, sets: a.sets, dish: a}
	if a.isWebDep {
		var (
			iWebDep IWebCookware
			err     error
		)
		iWebDep, err = any(dep).(IWebCookware).RequestParser(a.instance.(IDish), w, r)
		if err != nil {
			return nil, err
		}
		ctx.cookware = iWebDep.(D)
		if a.isTraceable {
			ctx.traceableDep = any(iWebDep).(ITraceableCookware)
		}
	} else {
		ctx.cookware = dep
		if a.isTraceable {
			ctx.traceableDep = any(ctx.cookware).(ITraceableCookware)
		}
	}
	return ctx, nil
}
