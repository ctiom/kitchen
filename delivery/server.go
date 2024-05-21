package delivery

import (
	"capnproto.org/go/capnp/v3"
	deliveryProto "cd.codes/galaxy/kitchen/delivery/proto"
	"context"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"sync"
	"sync/atomic"
	"time"
)

var (
	DefaultListenAddr      = "0.0.0.0"
	DefaultPubPort         = 19527
	DefaultRepPort         = 29527
	DefaultRepRecvThreads  = 5
	DefaultConcurrentLimit = 1000
	logErr                 = func(string, error, ...interface{}) {}
	logDebug               = func(string, ...interface{}) {}
)

const (
	CONNECTED_MSG_FLAG byte = 0
)

var (
	orderPool = sync.Pool{
		New: func() interface{} {
			return &Order{}
		},
	}
)

type node struct {
	url     string
	givenId uint32
}

type server struct {
	orderListener    func(context.Context, *Order)
	statusListener   func(*Status)
	cancelListen     context.CancelFunc
	localUrl         string
	mainUrl          string
	menuNames        []string
	requestCancelers []context.CancelFunc
	requestCh        chan *Order
	responseCh       chan *Deliverable
	peers            []node
}

func NewServer(localHostUrl, mainHostUrl string, menuNames []string) (ILogistic, error) {
	s := &server{
		localUrl:  localHostUrl,
		mainUrl:   mainHostUrl,
		menuNames: menuNames,
	}
	return s.Init()
}

func (s *server) Init() (ILogistic, error) {
	err := s.listenRequest()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *server) listenRequest() error {
	var (
		err       error
		url       = s.localUrl
		ctx       context.Context
		id        int32
		idPtr     = &id
		ringLimit = int32(DefaultRepRecvThreads * 100)
	)
	if url == "" {
		url = fmt.Sprintf("tcp://%s:%d", DefaultListenAddr, DefaultRepPort)
	}
	responder, err := zmq.NewSocket(zmq.REP)
	if err != nil {
		return err
	}
	err = responder.Connect(url)
	if err != nil {
		return err
	}
	s.requestCancelers = make([]context.CancelFunc, ringLimit)
	s.requestCh = make(chan *Order, DefaultConcurrentLimit)
	s.responseCh = make(chan *Deliverable, DefaultConcurrentLimit)
	ctx, s.cancelListen = context.WithCancel(context.Background())
	var (
		msgData  []byte
		order    = &Order{}
		capnMsg  *capnp.Message
		orderMsg deliveryProto.Order
		thisCtx  context.Context
	)
	for {
		for {
			msgData, err = responder.RecvBytes(0)
			if err != nil {
				logErr("Recv order", err)
				time.Sleep(time.Millisecond * 100)
			}
			switch msgData[0] {
			case CONNECTED_MSG_FLAG: // connect
				s.peers = append(s.peers, node{})
			}
			capnMsg, err = capnp.Unmarshal(msgData)
			if err != nil {
				logErr("Unmarshal order", err)
				continue
			}
			orderMsg, err = deliveryProto.ReadRootOrder(capnMsg)
			order.Input, err = orderMsg.Input()
			if err != nil {
				logErr("ReadRootOrder", err)
				continue
			}
			order.DishId = orderMsg.DishId()
			order.MenuId = orderMsg.MenuId()
			thisCtx, s.requestCancelers[atomic.AddInt32(idPtr, 1)%ringLimit] = context.WithDeadline(ctx, time.UnixMicro(orderMsg.Timeout()))

			s.orderListener(thisCtx, order)
		}
	}
	return nil
}

func (s *server) ListenOrder(f func(context.Context, *Order)) {

}
