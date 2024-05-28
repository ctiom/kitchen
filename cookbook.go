package kitchen

import (
	"fmt"
	"runtime/debug"
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
	concurrentLimit               *int32
	running                       *int32
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

func (r *cookbook[D, I, O]) ConcurrentLimit(limit int32) {
	if atomic.LoadInt32(r.concurrentLimit) < limit {
		defer func() {
			if atomic.LoadInt32(r.running) < limit {
				r.locker.TryLock()
				r.locker.Unlock()
			}
		}()
	}
	atomic.StoreInt32(r.concurrentLimit, limit)
}

func (r cookbook[D, I, O]) start(ctx IContext[D], input I, panicRecover bool) (sess *dishServing) {
	sess = r.newServing(input) //&dishServing{ctx: ctx, Input: input}
	if r.isTraceable {
		sess.tracerSpan = ctx.startTrace(r.fullName, TraceIdGenerator(), input)
	}
	if len(ctx.Session(sess)) == 1 {
		sess.unlocker = r.ifLockThis()
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

func (r *cookbook[D, I, O]) ifLock() func() {
	if limit := atomic.LoadInt32(r.concurrentLimit); limit != 0 {
		if atomic.AddInt32(r.running, 1) >= limit {
			r.locker.Lock()
		}
		return r.releaseLimit
	}
	return nil
}

func (r *cookbook[D, I, O]) ifLockThis() func() {
	var (
		unlock  func()
		unlocks = make([]func(), 0, len(r.inherited)+1)
	)
	for _, inherited := range r.inherited {
		unlock = inherited.ifLock()
		if unlock != nil {
			unlocks = append(unlocks, unlock)
		}
	}
	if unlock = r.ifLock(); unlock != nil {
		unlocks = append(unlocks, unlock)
	}
	if len(unlocks) == 0 {
		return nil
	} else if len(unlocks) == 1 {
		return unlocks[0]
	} else {
		return func() {
			for _, unlock := range unlocks {
				unlock()
			}
		}
	}
}

func (b cookbook[D, I, O]) Nodes() []IInstance {
	res := make([]IInstance, len(b.nodes))
	for i, n := range b.nodes {
		res[i] = n
	}
	return res
}

func (r *cookbook[D, I, O]) releaseLimit() {
	if running := atomic.AddInt32(r.running, -1); running == 0 || running == atomic.LoadInt32(r.concurrentLimit)-1 {
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
