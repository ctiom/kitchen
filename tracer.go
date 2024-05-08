package kitchen

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"time"
)

var (
	TraceIdGenerator = traceId
)

func traceId() int64 {
	return time.Now().UnixNano()
}

type ChainTraceableCookware []ITraceableCookware

func NewChainTraceableCookware(deps ...ITraceableCookware) *ChainTraceableCookware {
	var chain = ChainTraceableCookware(deps)
	return &chain
}

func (d ChainTraceableCookware) StartTrace(ctx context.Context, id string, spanName string, input any) (context.Context, ITraceSpan) {
	var (
		spans = make([]ITraceSpan, len(d))
		span  ITraceSpan
	)
	for i, dep := range d {
		ctx, span = dep.StartTrace(ctx, id, spanName, input)
		spans[i] = span
	}
	return ctx, &chainTraceSpan{spans: spans}
}

type chainTraceSpan struct {
	spans []ITraceSpan
}

func (c chainTraceSpan) logSideEffect(ctx context.Context, instanceName string, toLog []any) (context.Context, ITraceSpan) {
	var (
		subSpan = &chainTraceSpan{spans: make([]ITraceSpan, len(c.spans))}
	)
	for i, span := range c.spans {
		ctx, subSpan.spans[i] = span.logSideEffect(ctx, instanceName, toLog)
	}
	return ctx, subSpan
}

func (c chainTraceSpan) Detail() any {
	var details = make([]any, 0, len(c.spans))
	for i, span := range c.spans {
		details[i] = span.Detail()
	}
	return details
}

func (c chainTraceSpan) End(output any, err error) {
	for _, span := range c.spans {
		span.End(output, err)
	}
}

func (c chainTraceSpan) AddEvent(name string, attrSets ...map[string]any) {
	for _, span := range c.spans {
		span.AddEvent(name, attrSets...)
	}
}

func (c chainTraceSpan) SetAttributes(key string, value any) {
	for _, span := range c.spans {
		span.SetAttributes(key, value)
	}
}

func (c chainTraceSpan) Raw() any {
	var raws = make([]any, 0, len(c.spans))
	for _, span := range c.spans {
		raws = append(raws, span.Raw())
	}
	return raws
}

type ZeroLogTraceableCookware struct {
	Logger *zerolog.Logger
}

type zeroLogTraceSpan struct {
	logger *zerolog.Logger
}

func NewZeroLogTraceableCookware(l *zerolog.Logger) *ZeroLogTraceableCookware {
	return &ZeroLogTraceableCookware{Logger: l}
}

func (d ZeroLogTraceableCookware) StartTrace(ctx context.Context, id string, spanName string, input any) (context.Context, ITraceSpan) {
	if d.Logger.GetLevel() > zerolog.DebugLevel {
		return ctx, &zeroLogTraceSpan{d.Logger}
	}
	*d.Logger = d.Logger.With().Str("dishId", id).Logger()
	logger := d.Logger.With().Str("action", spanName).Logger()
	logger.Debug().Interface("input", input).Msg("call")
	return ctx, &zeroLogTraceSpan{logger: &logger}
}

func (c zeroLogTraceSpan) logSideEffect(ctx context.Context, instanceName string, toLog []any) (context.Context, ITraceSpan) {
	if c.logger.GetLevel() > zerolog.DebugLevel {
		return ctx, &zeroLogTraceSpan{c.logger}
	}
	logger := c.logger.With().Str("sideEffect", instanceName).Logger()
	logger.Debug().Interface("desc", toLog).Msg("call")
	return ctx, &zeroLogTraceSpan{logger: &logger}
}

func (d zeroLogTraceSpan) Detail() any {
	return nil
}

func (d zeroLogTraceSpan) End(output any, err error) {
	d.logger.Debug().Str("output", fmt.Sprintf("%v", output)).Err(err).Msg("return")
}

func (d zeroLogTraceSpan) AddEvent(name string, attrSets ...map[string]any) {
	d.logger.Debug().Interface("event", name).Interface("attrs", attrSets).Msg("event")
}

func (d zeroLogTraceSpan) SetAttributes(key string, value any) {
	d.logger.Debug().Interface(key, value).Msg("attrs")
}

func (d zeroLogTraceSpan) Raw() any {
	return d.logger
}

type otelTraceableCookware struct {
	t trace.Tracer
}

func NewOtelTraceableCookware(t trace.Tracer) *otelTraceableCookware {
	return &otelTraceableCookware{t: t}
}

func (d otelTraceableCookware) StartTrace(ctx context.Context, id string, spanName string, input any) (context.Context, ITraceSpan) {
	ctx, span := d.t.Start(ctx, spanName, trace.WithAttributes(attribute.String("dishId", id), attribute.String("input", fmt.Sprintf("%v", input))))
	return ctx, &otelTraceSpan{span: span, t: d.t}
}

type otelTraceSpan struct {
	span trace.Span
	t    trace.Tracer
}

func (o otelTraceSpan) logSideEffect(ctx context.Context, instanceName string, toLog []any) (context.Context, ITraceSpan) {
	ctx, span := o.t.Start(ctx, "sideEffect:"+instanceName, trace.WithAttributes(attribute.String("desc", fmt.Sprintf("%v", toLog))))
	return ctx, &otelTraceSpan{span: span, t: o.t}
}

func (o otelTraceSpan) Detail() any {
	return o.span.SpanContext()
}

func (o otelTraceSpan) End(output any, err error) {
	o.span.SetAttributes(attribute.String("output", fmt.Sprintf("%v", output)), attribute.String("error", fmt.Sprintf("%v", err)))
	o.span.End()
}

func (o *otelTraceSpan) AddEvent(name string, attrSets ...map[string]any) {
	if len(attrSets) == 0 {
		o.span.AddEvent(name)
		return
	}
	o.span.AddEvent(name, trace.WithAttributes(o.makeAttr(attrSets...)...))
}

func (o otelTraceSpan) SetAttributes(key string, value any) {
	o.span.SetAttributes(o.makeKv(key, value))
}

func (o otelTraceSpan) Raw() any {
	return o.span
}

func (o otelTraceSpan) makeAttr(attrSets ...map[string]any) []attribute.KeyValue {
	var kvs []attribute.KeyValue
	for _, attrs := range attrSets {
		for k, v := range attrs {
			kvs = append(kvs, o.makeKv(k, v))
		}
	}
	return kvs
}

func (o otelTraceSpan) makeKv(k string, v any) attribute.KeyValue {
	switch v.(type) {
	case string:
		return attribute.String(k, v.(string))
	case uint:
		return attribute.Int(k, int(v.(uint)))
	case uint32:
		return attribute.Int64(k, int64(v.(uint32)))
	case uint64:
		return attribute.Int64(k, int64(v.(uint64)))
	case int:
		return attribute.Int(k, v.(int))
	case int32:
		return attribute.Int64(k, v.(int64))
	case int64:
		return attribute.Int64(k, v.(int64))
	case float64:
		return attribute.Float64(k, v.(float64))
	case bool:
		return attribute.Bool(k, v.(bool))
	default:
		return attribute.String(k, fmt.Sprintf("%v", v))
	}
}
