package delivery

import (
	"context"
	"github.com/stretchr/testify/assert"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

var (
	mainServers = make([]ILogistic, 1)
	nodeServers = make([]ILogistic, 3)
)

func init() {
	var (
		err error
	)
	DefaultStatusBroadcastInterval = time.Second * 10
	for i, l := 0, len(mainServers); i < l; i++ {
		mainServers[i] = NewServer("tcp://127.0.0.1", uint16(10000+i), "", 0)

		mainServers[i].SetOrderHandlerPerMenu([]func(ctx context.Context, order *Order){func(ctx context.Context, order *Order) {
			//time.Sleep(time.Millisecond * 50)
			order.Response(order.Input, nil)
		}})
		err = mainServers[i].Init()
		if err != nil {
			panic(err)
		}
	}
	time.Sleep(time.Millisecond * 100)
	for i, l := 0, len(nodeServers); i < l; i++ {
		nodeServers[i] = NewServer("tcp://127.0.0.1", uint16(20000+i), "tcp://127.0.0.1", uint16(10000+i%len(mainServers)))
		err = nodeServers[i].Init()
		if err != nil {
			panic(err)
		}
	}
	time.Sleep(time.Millisecond * 1000)
}

func TestOrder(t *testing.T) {
	resp, err := nodeServers[0].(*server).peers[0].OrderRequestAsClient(context.Background(), 0, 0, []byte("hello"))
	assert.Nil(t, err)
	assert.Equal(t, []byte("hello"), resp)
}

func BenchmarkOrder(b *testing.B) {
	ii := int32(0)
	b.SetParallelism(1000)
	b.RunParallel(func(pb *testing.PB) {
		var (
			err    error
			s            = atomic.AddInt32(&ii, 1) % int32(len(nodeServers))
			i      int64 = 0
			data   []byte
			res    []byte
			ctx    = context.Background()
			server = nodeServers[s].(*server)
		)
		for pb.Next() {
			data = []byte("hello" + strconv.FormatInt(i, 10))
			res, err = server.peers[0].OrderRequestAsClient(ctx, 0, 0, data)
			if err != nil {
				b.Fatal(err)
			}
			if string(data) != string(res) {
				b.Fatalf("expected %s, got %s", string(data), string(res))
			}
			i++
		}
	})
}

func TestShutdown(t *testing.T) {
	leader := 0
	mainServers[0].Shutdown()
	assert.False(t, mainServers[0].IsLeader())
	time.Sleep(time.Second * 5)
	for i, l := 0, len(nodeServers); i < l; i++ {
		if nodeServers[i].IsLeader() {
			leader++
		}
	}
	assert.Equal(t, 1, leader)
}
