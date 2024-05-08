package kitchen

import (
	"fmt"
	"math/rand"
	"runtime/debug"
	"strconv"
	"sync"
	"sync/atomic"
)

//func (e *cookbook[M]) BeforeExec(handler BeforeListenHandler[M]) *cookbook[M] {
//	e.beforeListenHandlers = append(e.beforeListenHandlers, handler)
//	return e
//}

type cookbook[D ICookware, I any, O any] struct {
	//beforeListenHandlers []BeforeListenHandler[M]
	instance                      IInstance
	afterListenHandlers           []AfterListenHandlers[D, I, O]
	afterListenHandlersExtra      [][]any
	asyncAfterListenHandlers      []AfterListenHandlers[D, I, O]
	asyncAfterListenHandlersExtra [][]any
	inherited                     []iCookbook[D]
	concurrent                    int64
	running                       int64
	concurrentCache               *int64
	locker                        *sync.Mutex
	nodes                         []iCookbook[D]
	fullName                      string
	isTraceable                   bool
	isInheritableCookware         bool
	isWebWrapperCookware          bool
}

func (r cookbook[D, I, O]) Menu() IMenu {
	return r.instance.Menu()
}

func (b cookbook[D, I, O]) isTraceableDep() bool {
	return b.isTraceable
}

func (b cookbook[D, I, O]) isInheritableDep() bool {
	return b.isInheritableCookware
}

func (r *cookbook[D, I, O]) AfterExec(handler AfterListenHandlers[D, I, O], toLog ...any) *cookbook[D, I, O] {
	return r.AfterCook(handler, toLog...)
}

func (r *cookbook[D, I, O]) AfterCook(handler AfterListenHandlers[D, I, O], toLog ...any) *cookbook[D, I, O] {
	r.afterListenHandlers = append(r.afterListenHandlers, handler)
	r.afterListenHandlersExtra = append(r.afterListenHandlersExtra, toLog)
	return r
}

func (r *cookbook[D, I, O]) AfterExecAsync(handler AfterListenHandlers[D, I, O], toLog ...any) *cookbook[D, I, O] {
	return r.AfterCookAsync(handler, toLog...)
}

func (r *cookbook[D, I, O]) AfterCookAsync(handler AfterListenHandlers[D, I, O], toLog ...any) *cookbook[D, I, O] {
	r.asyncAfterListenHandlers = append(r.asyncAfterListenHandlers, handler)
	r.asyncAfterListenHandlersExtra = append(r.asyncAfterListenHandlersExtra, toLog)
	return r
}

func (r *cookbook[D, I, O]) inherit(ev ...iCookbook[D]) {
	r.inherited = append(r.inherited, ev...)
}

func (r cookbook[D, I, O]) emitAfterExec(ctx IContext[D], input, output any, err error) {
	r.emitAfterCook(ctx, input, output, err)
}

func (r cookbook[D, I, O]) emitAfterCook(ctx IContext[D], input, output any, err error) {
	if l := len(r.asyncAfterListenHandlers); l+len(r.afterListenHandlers) != 0 {
		if l != 0 {
			ctx.servedWeb()
			go func() {
				var (
					cbCtx = ctx
					t     ITraceSpan
				)
				for i, handler := range r.asyncAfterListenHandlers {
					cbCtx, t = cbCtx.logSideEffect(r.instance.Name(), r.asyncAfterListenHandlersExtra[i])
					handler(cbCtx, input.(I), output.(O), err)
					if t != nil {
						t.End(nil, nil)
					}
				}
			}()
		}
		var (
			cbCtx = ctx
			t     ITraceSpan
		)
		for i, handler := range r.afterListenHandlers {
			cbCtx, t = cbCtx.logSideEffect(r.instance.Name(), r.afterListenHandlersExtra[i])
			handler(cbCtx, input.(I), output.(O), err)
			if t != nil {
				t.End(nil, nil)
			}
		}
	}
	for _, ev := range r.inherited {
		ev.emitAfterCook(ctx, input, output, err)
	}
}

func (r cookbook[D, I, O]) concurrentLimit() int64 {
	return r.concurrent
}

func (r *cookbook[D, I, O]) ConcurrentLimit(limit int64) {
	r.concurrent = limit
}

// get limit
func (r *cookbook[D, I, O]) getConcurrentLimit() int64 {
	if r.concurrentCache == nil {
		var limit = r.concurrentLimit()
		if limit == 0 {
			for _, ev := range r.inherited {
				limit = ev.concurrentLimit()
				if limit != 0 {
					break
				}
			}
		}
		r.concurrentCache = &limit
		r.locker = &sync.Mutex{}
	}
	return *r.concurrentCache
}

func (r *cookbook[D, I, O]) start(ctx IContext[D], input I, panicRecover bool) (sess *dishServing) {
	sess = r.newServing(input) //&dishServing{ctx: ctx, Input: input}
	if r.isTraceable {
		sess.tracerSpan = ctx.startTrace(r.fullName, strconv.FormatInt(rand.Int63n(999999999), 36), input)
	}
	if len(ctx.Session(sess)) == 1 {
		var limit = r.getConcurrentLimit()
		if limit != 0 {
			if atomic.AddInt64(&r.running, 1) >= limit {
				r.locker.Lock()
			}
			sess.unlocker = r.releaseLimit
		}
		if panicRecover {
			defer func() {
				if rec := recover(); rec != nil {
					if r.isTraceable {
						sess.tracerSpan.AddEvent("panic", map[string]any{"panic": rec, "stack": string(debug.Stack())})
					} else {
						fmt.Printf("panicRecover from panic: \n%v\n%s", r, string(debug.Stack()))
					}
				}
			}()
		}
	}
	return
}
func (r *cookbook[D, I, O]) releaseLimit() {
	if running := atomic.AddInt64(&r.running, -1); running == 0 || running == r.getConcurrentLimit()-1 {
		_ = r.locker.TryLock()
		r.locker.Unlock()
	}
}

func (r *cookbook[D, I, O]) newServing(input I) *dishServing {
	return &dishServing{Action: r.instance.(IDish), Input: input}
}

type dishServing struct {
	Action     IDish
	Input      any
	Output     any
	Error      error
	Finish     bool
	unlocker   func()
	tracerSpan ITraceSpan
}

func (node *dishServing) finish(output any, err error) {
	if node.unlocker != nil {
		node.unlocker()
	}
	if node.tracerSpan != nil {
		node.tracerSpan.End(output, err)
	}
	node.Finish = true
	node.Output = output
	node.Error = err
}

func (node *dishServing) Record() (IDish, bool, any, any, error) {
	return node.Action, node.Finish, node.Input, node.Output, node.Error
}
