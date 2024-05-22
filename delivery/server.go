package delivery

import (
	"encoding/binary"

	//deliveryProto "github.com/ctiom/kitchen/delivery/proto"
	"context"
	"fmt"
	zmq "github.com/pebbe/zmq4"
	"sync"
	"time"
)

var (
	DefaultListenAddr      = "0.0.0.0"
	DefaultPubPort         = 19527
	DefaultRepPort         = 29527
	DefaultRepRecvThreads  = 5
	DefaultConcurrentLimit = 1000
	logErr                 = func(str string, err error, arg ...interface{}) {
		fmt.Println(str, err, arg)
	}
	logDebug = func(args ...interface{}) {
		fmt.Println(args...)
	}
)

const (
	MSG_FLAG_CONNECTED     byte = 0
	MSG_FLAG_CONNECTED_ACK byte = 1
)

var (
	orderPool = sync.Pool{
		New: func() interface{} {
			return &Order{}
		},
	}
)

type node struct {
	server    *server
	req       *zmq.Socket
	url       string
	localId   uint32
	foreignId uint32
}

func (n *node) initRequest(retry uint32) {
	var (
		err error
	)
	if retry != 0 {
		if retry > 100 {
			retry = 100
		}
		time.Sleep(time.Millisecond * 100 * time.Duration(retry))
	}
	n.req, err = zmq.NewSocket(zmq.REQ)
	if err != nil {
		logErr("NewSocket", err)
		return
	}
	logDebug("connect", n.url)
	err = n.req.Connect(n.url)
	if err != nil {
		logErr("Connect", err)
		n.initRequest(retry + 1)
		return
	}
	logDebug("send", n.url)
	_, err = n.req.SendBytes(append([]byte{MSG_FLAG_CONNECTED}, []byte(n.server.localUrl)...), 0)
	if err != nil {
		logErr("node SendBytes", err)
		n.initRequest(retry + 1)
		return
	}
	logDebug("wait", n.url)
	resp, err := n.req.RecvBytes(0)
	if err != nil {
		logErr("node RecvBytes", err)
		n.initRequest(retry + 1)
		return
	}
	logDebug("got", n.url, resp)
	if len(resp) != 5 || resp[0] != MSG_FLAG_CONNECTED_ACK {
		logErr("node invalid ack", nil, resp)
		n.initRequest(retry + 1)
		return
	}
	n.foreignId = binary.BigEndian.Uint32(resp[1:])
	n.server.kitchenReady(n)
}

type server struct {
	sync.Mutex
	orderListener    func(context.Context, *Order)
	statusListener   func(*Status)
	cancelListen     context.CancelFunc
	localUrl         string
	mainUrl          string
	menuNames        []string
	requestCancelers []context.CancelFunc
	requestCh        chan *Order
	responseCh       chan *Deliverable
	peers            []*node
}

func (s *server) ListenStatus(f func(*Status)) {
	//TODO implement me
	panic("implement me")
}

func (s *server) SwitchHost(host string) {
	//todo get all host list

	//dummy
	s.peers = append(s.peers, &node{
		server: s,
		url:    host,
	})

	for _, node := range s.peers {
		node.initRequest(0)
	}

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
		err error
		url = s.localUrl
		//ctx       context.Context
		//id        int32
		//idPtr     = &id
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
	_, s.cancelListen = context.WithCancel(context.Background())
	go func() {
		var (
			msgData []byte
			//order   = &Order{}
			//capnMsg *capnp.Message
			//orderMsg deliveryProto.Order
			//thisCtx context.Context
		)
		logDebug("server start", url)
		for {
			msgData, err = responder.RecvBytes(0)
			if err != nil {
				logErr("Recv order", err)
			}
			logDebug("received", msgData)
			switch msgData[0] {
			case MSG_FLAG_CONNECTED: // connect
				_, err = responder.SendBytes(s.addNodeAndReply(string(msgData[0:])), 0)
				if err != nil {
					logErr("Send connected ack", err)
				}

			}
			//capnMsg, err = capnp.Unmarshal(msgData)
			//if err != nil {
			//	logErr("Unmarshal order", err)
			//	continue
			//}
			//orderMsg, err = deliveryProto.ReadRootOrder(capnMsg)
			//order.Input, err = orderMsg.Input()
			//if err != nil {
			//	logErr("ReadRootOrder", err)
			//	continue
			//}
			//order.DishId = orderMsg.DishId()
			//order.MenuId = orderMsg.MenuId()
			//thisCtx, s.requestCancelers[atomic.AddInt32(idPtr, 1)%ringLimit] = context.WithDeadline(ctx, time.UnixMicro(orderMsg.Timeout()))

			//s.orderListener(thisCtx, order)
		}
	}()
	return nil
}

func (s *server) ListenOrder(f func(context.Context, *Order)) {

}

func (s *server) addNodeAndReply(url string) []byte {
	s.Lock()
	node := &node{
		server:  s,
		localId: uint32(len(s.peers)),
		url:     url,
	}
	s.peers = append(s.peers, node)
	s.Unlock()
	go node.initRequest(0)
	reply := make([]byte, 5)
	reply[0] = MSG_FLAG_CONNECTED_ACK
	binary.BigEndian.PutUint32(reply[1:], node.localId)
	return reply
}

func (s *server) kitchenReady(n *node) {

}
