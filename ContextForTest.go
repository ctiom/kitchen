package kitchen

import (
	"context"
	"net/http"
)

type ContextForTest[D ICookware] struct {
	context.Context
	SessionServed []IDishServe
	DummyCookware D
	DummyDish     iDish[D]
	DummySets     []iSet[D]
	DummyMenu     iMenu[D]
	webBundle     *webBundle
}

func (c *ContextForTest[D]) Session(serve ...IDishServe) []IDishServe {
	c.SessionServed = append(c.SessionServed, serve...)
	return c.SessionServed
}

func (c ContextForTest[D]) SetCtx(ctx context.Context) {
	c.Context = ctx
}

func (c ContextForTest[D]) RawCookware() ICookware {
	return c.DummyCookware
}

func (c ContextForTest[D]) Menu() iMenu[D] {
	return c.DummyMenu
}

func (c ContextForTest[D]) Sets() []iSet[D] {
	return c.DummySets
}

func (c ContextForTest[D]) Dish() iDish[D] {
	return c.DummyDish
}

func (c ContextForTest[D]) Dependency() D {
	return c.DummyCookware
}

func (c ContextForTest[D]) Cookware() D {
	return c.DummyCookware
}

func (c ContextForTest[D]) traceableCookware() ITraceableCookware {
	return nil
}

func (c ContextForTest[D]) startTrace(name string, id string, input any) ITraceSpan {
	return nil
}

func (c ContextForTest[D]) logSideEffect(instanceName string, toLog []any) (IContext[D], ITraceSpan) {
	return nil, nil
}

func (c ContextForTest[D]) FromWeb() *webBundle {
	return c.webBundle
}

func (c *ContextForTest[D]) SetWebBundle(body []byte, req *http.Request, resp http.ResponseWriter) {
	c.webBundle = &webBundle{
		RequestBody: body,
		Request:     req,
		Response:    resp,
	}
}

func (c ContextForTest[D]) TraceSpan() ITraceSpan {
	return nil
}

func (c ContextForTest[D]) GetCtx() context.Context {
	return c.Context
}

func (c *ContextForTest[D]) servedWeb() {
}

type PipelineContextForTest[D IPipelineCookware[M], M IPipelineModel] struct {
	ContextForTest[D]
	DummyTx IDbTx
}

func (b PipelineContextForTest[D, M]) Tx() IDbTx {
	return b.DummyTx
}

func (b PipelineContextForTest[D, M]) Pipeline() iPipeline[D, M] {
	return b.DummyMenu.(iPipeline[D, M])
}

func (b PipelineContextForTest[D, M]) Stage() iPipelineStage[D, M] {
	return b.DummySets[0].(iPipelineStage[D, M])
}

func (c *PipelineContextForTest[D, M]) logSideEffect(instanceName string, toLog []any) (IContext[D], ITraceSpan) {
	return c, nil
}
