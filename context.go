package kitchen

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
)

type Context[D ICookware] struct {
	context.Context
	menu         iMenu[D]
	sets         []iSet[D]
	dish         iDish[D]
	session      []IDishServe
	sideEffects  []IInstance
	cookware     D
	traceableDep ITraceableCookware
	inherited    IContextWithSession
	node         IDishServe
	tracerSpan   ITraceSpan
	WebBundle    *webBundle
	webContext   *webContext
}

type webBundle struct {
	RequestBody []byte
	Request     *http.Request
	Response    http.ResponseWriter
}

func (w *webBundle) GetRequestUrlParams() map[string]string {
	return mux.Vars(w.Request)
}

func (w *webBundle) GetRequestQueryParams() (queryParam map[string]string) {
	queryParam = make(map[string]string)
	for k, v := range w.Request.URL.Query() {
		queryParam[k] = v[0]
	}
	return
}

type webContext struct {
	context.Context
	hasServedWeb bool
	ch           chan struct{}
	err          error
}

func newWebContext(ctx context.Context) *webContext {
	wc := &webContext{Context: ctx}
	wc.ch = make(chan struct{})
	go func() {
		<-ctx.Done()
		if !wc.hasServedWeb {
			close(wc.ch)
		}
	}()
	return wc
}

func (c *webContext) servedWeb() {
	c.hasServedWeb = true
	c.err = c.Context.Err()
}

func (c webContext) Done() <-chan struct{} {
	return c.ch
}

func (c *webContext) Err() error {
	if c.hasServedWeb {
		return c.err
	}
	return c.Context.Err()
}

func (c *Context[D]) SetCtx(ctx context.Context) {
	c.Context = ctx
}

func (c Context[D]) Menu() iMenu[D] {
	return c.menu
}

func (c Context[D]) Sets() []iSet[D] {
	return c.sets
}

func (c Context[D]) Dish() iDish[D] {
	return c.dish
}

func (c Context[D]) Dependency() D {
	return c.cookware
}

func (c Context[D]) RawCookware() ICookware {
	return c.cookware
}

func (c Context[D]) Cookware() D {
	return c.cookware
}

func (c Context[D]) traceableCookware() ITraceableCookware {
	return c.traceableDep
}

func (c Context[D]) FromWeb() *webBundle {
	return c.WebBundle
}

func (c Context[D]) GetCtx() context.Context {
	return c.Context
}

func (c *Context[D]) startTrace(name string, id string, input any) ITraceSpan {
	c.Context = context.WithValue(c.Context, "kitchenDishId", id)
	if c.traceableDep != nil {
		c.Context, c.tracerSpan = c.traceableDep.StartTrace(c.Context, id, name, input)
		if c.WebBundle != nil && len(c.WebBundle.RequestBody) != 0 {
			c.tracerSpan.SetAttributes("webReqBody", string(c.WebBundle.RequestBody))
		}
		return c.tracerSpan
	}
	return nil
}

func (c *Context[D]) logSideEffect(instanceName string, toLog []any) (IContext[D], ITraceSpan) {
	if c.tracerSpan != nil {
		var (
			cc  = *c
			ccc = &cc
		)
		ccc.Context, ccc.tracerSpan = ccc.tracerSpan.logSideEffect(c.Context, instanceName, toLog)
		return ccc, ccc.tracerSpan
	}
	return c, nil
}

func (c *Context[D]) Session(nodes ...IDishServe) []IDishServe {
	if len(nodes) != 0 {
		c.node = nodes[0]
		if c.inherited != nil {
			return c.inherited.Session(nodes...)
		}
		c.session = append(c.session, nodes...)
	} else {
		if c.inherited != nil {
			return c.inherited.Session(nodes...)
		}
	}
	return c.session
}

func (c *Context[D]) TraceSpan() ITraceSpan {
	return c.tracerSpan
}

func (c *Context[D]) servedWeb() {
	if c.webContext != nil {
		c.webContext.servedWeb()
	}
}

type PipelineContext[D IPipelineCookware[M], M IPipelineModel] struct {
	Context[D]
	tx IDbTx
}

func (b PipelineContext[D, M]) Tx() IDbTx {
	return b.tx
}

func (b PipelineContext[D, M]) Pipeline() iPipeline[D, M] {
	return b.menu.(iPipeline[D, M])
}

func (b PipelineContext[D, M]) Stage() iPipelineStage[D, M] {
	return b.sets[0].(iPipelineStage[D, M])
}

func (c *PipelineContext[D, M]) logSideEffect(instanceName string, toLog []any) (IContext[D], ITraceSpan) {
	c.Context.logSideEffect(instanceName, toLog)
	return c, nil
}
