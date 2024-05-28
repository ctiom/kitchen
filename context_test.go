package kitchen

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestWebContextNormal(t *testing.T) {
	dummyWebCtx, cancel := context.WithCancel(context.Background())
	webCtx := NewWebContext(context.WithValue(dummyWebCtx, "test1", 1), nil, nil)
	ctx := context.WithValue(webCtx, "test2", 2)
	go func() {
		done := ctx.Done()
		<-done
		fmt.Println("done")
	}()
	webCtx.servedWeb()
	cancel()
	time.Sleep(1 * time.Second)
	assert.Equal(t, 1, ctx.Value("test1"))
	assert.Equal(t, 2, ctx.Value("test2"))
	fmt.Println("ok")

}

func TestWebContextAbnormal(t *testing.T) {
	dummyWebCtx, cancel := context.WithCancel(context.Background())
	ctx := NewWebContext(context.WithValue(dummyWebCtx, "test", 1), nil, nil)
	go func() {
		done := ctx.Done()
		<-done
		fmt.Println("done")
	}()
	cancel()
	ctx.servedWeb()
	assert.NotNil(t, ctx.Err())

}
