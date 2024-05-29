package benchmark

import (
	"context"
	"encoding/binary"
	pb "github.com/go-preform/kitchen/examples/proto"
	zmqGo "github.com/go-zeromq/zmq4"
	vtgrpc "github.com/planetscale/vtprotobuf/codec/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"testing"
)

type server struct {
	pb.UnimplementedGreeterServer
}

// SayHello implements helloworld.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	return &pb.HelloReply{Response: in.Request}, nil
}

var replyChan = make([]chan zmqGo.Msg, 1000000)

func init() {

	go func() {
		lis, err := net.Listen("tcp", "127.0.0.1:15559")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		s := grpc.NewServer()
		pb.RegisterGreeterServer(s, &server{})
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	go func() {
		lis, err := net.Listen("tcp", "127.0.0.1:15560")
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
		encoding.RegisterCodec(vtgrpc.Codec{})
		s := grpc.NewServer()
		pb.RegisterGreeterServer(s, &server{})
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	go func() {
		responder := zmqGo.NewRep(context.Background())
		err := responder.Listen("tcp://127.0.0.1:15562")
		if err != nil {
			panic(err)
		}
		var (
			toReply = zmqGo.NewMsg([]byte("World"))
		)
		for {
			msg, err := responder.Recv()
			if err != nil {
				panic(err)
			}
			if string(msg.Bytes()) != "Hello" {
				panic("Expected 'Hello', got '" + string(msg.Bytes()) + "'")
			}
			err = responder.Send(toReply)
			if err != nil {
				panic(err)
			}
		}
	}()
	go func() {
		responder := zmqGo.NewRouter(context.Background())
		err := responder.Listen("tcp://127.0.0.1:15563")
		if err != nil {
			panic(err)
		}
		var (
			toReply = []byte("World")
			id      []byte
		)
		for {
			msg, err := responder.Recv()
			if err != nil {
				panic(err)
			}
			id = msg.Frames[0]
			if string(msg.Frames[2]) != "Hello" {
				panic("Expected 'Hello', got '" + string(msg.Frames[2]) + "'")
			}
			err = responder.Send(zmqGo.NewMsgFrom(id, []byte(""), toReply))
			if err != nil {
				panic(err)
			}
		}
	}()

	for i := 0; i < 1000000; i++ {
		replyChan[i] = make(chan zmqGo.Msg, 10)
	}

	go func() {
		responder := zmqGo.NewRep(context.Background())
		err := responder.Listen("tcp://127.0.0.1:15565")
		if err != nil {
			panic(err)
		}
		var (
			toReply = zmqGo.NewMsg([]byte("0000World"))
		)
		for {
			msg, err := responder.Recv()
			if err != nil {
				panic(err)
			}
			toReply.Frames[0][0] = msg.Frames[0][0]
			toReply.Frames[0][1] = msg.Frames[0][1]
			toReply.Frames[0][2] = msg.Frames[0][2]
			toReply.Frames[0][3] = msg.Frames[0][3]
			replyChan[binary.BigEndian.Uint32(msg.Frames[0])] <- toReply
		}
	}()

	go func() {
		responder := zmqGo.NewRep(context.Background())
		err := responder.Listen("tcp://127.0.0.1:15564")
		if err != nil {
			panic(err)
		}
		repChan := make(chan zmqGo.Msg, 10000)
		for i := 0; i < 100; i++ {
			go func() {
				req := zmqGo.NewReq(context.Background())
				err := req.Dial("tcp://127.0.0.1:15565")
				if err != nil {
					panic(err)
				}
				var (
					msg zmqGo.Msg
				)
				for {
					msg = <-repChan
					err = req.Send(msg)
					if err != nil {
						panic(err)
					}
				}
			}()
		}
		for {
			msg, err := responder.Recv()
			if err != nil {
				panic(err)
			}
			if string(msg.Frames[0][4:]) != "Hello" {
				panic("Expected 'Hello', got '" + string(msg.Frames[1]) + "'")
			}
			repChan <- msg
		}
	}()

	//go func() {
	//	ctx, _ := zmq.NewContext()
	//	defer ctx.Term()
	//
	//	//  Socket to talk to clients
	//	publisher, _ := ctx.NewSocket(zmq.REP)
	//	defer publisher.Close()
	//	publisher.SetSndhwm(1100000)
	//	publisher.Bind("tcp://127.0.0.1:5561")
	//	var (
	//		toReply = "World"
	//	)
	//	for {
	//		//  Wait for next request from client
	//		msg, err := publisher.Recv(0)
	//		//println("Received request: [", msg, "]")
	//		if err != nil {
	//			panic(err)
	//		}
	//		if msg != "Hello" {
	//			panic("Expected 'Hello', got '" + msg + "'")
	//		}
	//
	//		//  Do some 'work'
	//		//  Send reply back to client
	//		publisher.Send(toReply, 0)
	//	}
	//}()
}

func BenchmarkGrpc(b *testing.B) {
	b.SetParallelism(100)
	conn, err := grpc.Dial("127.0.0.1:15559", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	b.RunParallel(func(pbt *testing.PB) {
		c := pb.NewGreeterClient(conn)
		var (
			ctx   = context.Background()
			req   = &pb.HelloRequest{Request: "Hello"}
			reply *pb.HelloReply
		)
		for pbt.Next() {
			reply, err = c.SayHello(ctx, req)
			if err != nil {
				log.Fatalf("could not greet: %v", err)
			}
			if reply.Response != "Hello" {
				b.Fatalf("Expected reply to be 'World', got '%s'", reply.Response)
			}
		}
	})

}
func BenchmarkGrpcVt(b *testing.B) {
	b.SetParallelism(100)
	conn, err := grpc.Dial("127.0.0.1:15560", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	b.RunParallel(func(pbt *testing.PB) {
		c := pb.NewGreeterClient(conn)
		var (
			ctx   = context.Background()
			req   = &pb.HelloRequest{Request: "Hello"}
			reply *pb.HelloReply
		)
		for pbt.Next() {
			reply, err = c.SayHello(ctx, req)
			if err != nil {
				log.Fatalf("could not greet: %v", err)
			}
			if reply.Response != "Hello" {
				b.Fatalf("Expected reply to be 'World', got '%s'", reply.Response)
			}
		}
	})

}

//func BenchmarkZMQ4(b *testing.B) {
//	b.RunParallel(func(pbt *testing.PB) {
//		ctx, _ := zmq.NewContext()
//		defer ctx.Term()
//
//		//  Socket to talk to clients
//		requester, _ := ctx.NewSocket(zmq.REQ)
//		defer requester.Close()
//		requester.Connect("tcp://127.0.0.1:5561")
//
//		for pbt.Next() {
//			//  Send request
//			requester.Send("Hello", 0)
//
//			//  Get the reply.
//			reply, _ := requester.Recv(0)
//			if reply != "World" {
//				b.Fatalf("Expected reply to be 'World', got '%s'", reply)
//			}
//		}
//	})
//}

type ZMQGo struct {
	req  zmqGo.Socket
	lock sync.Mutex
}

func (z *ZMQGo) Send(msg zmqGo.Msg) error {
	z.lock.Lock()
	//  Send request
	return z.req.Send(msg)
}

func (z *ZMQGo) Recv() (zmqGo.Msg, error) {
	//  Get the reply.
	reply, err := z.req.Recv()
	z.lock.Unlock()
	return reply, err
}

func BenchmarkZMQGo(b *testing.B) {
	b.SetParallelism(1000)
	req := make([]*ZMQGo, 100)
	for i := 0; i < 100; i++ {
		req[i] = &ZMQGo{req: zmqGo.NewReq(context.Background())}
		err := req[i].req.Dial("tcp://127.0.0.1:15562")
		if err != nil {
			panic(err)
		}
	}
	i := new(int32)
	b.RunParallel(func(pbt *testing.PB) {

		var (
			req   = req[atomic.AddInt32(i, 1)%100]
			toReq = zmqGo.NewMsg([]byte("Hello"))
		)

		for pbt.Next() {
			//  Send request
			req.Send(toReq)

			//  Get the reply.
			reply, _ := req.Recv()
			if string(reply.Bytes()) != "World" {
				b.Fatalf("Expected reply to be 'World', got '%s'", reply)
			}
		}
	})
}

func BenchmarkZMQGoRouter(b *testing.B) {
	b.SetParallelism(1000)
	req := make([]*ZMQGo, 100)
	for i := 0; i < 100; i++ {
		req[i] = &ZMQGo{req: zmqGo.NewDealer(context.Background())}
		err := req[i].req.Dial("tcp://127.0.0.1:15563")
		if err != nil {
			panic(err)
		}
	}
	i := new(int32)
	b.RunParallel(func(pbt *testing.PB) {

		var (
			req   = req[atomic.AddInt32(i, 1)%100]
			toReq = zmqGo.NewMsgFrom([]byte(""), []byte("Hello"))
		)

		for pbt.Next() {
			//  Send request
			req.Send(toReq)

			//  Get the reply.
			reply, _ := req.Recv()
			if string(reply.Bytes()) != "World" {
				b.Fatalf("Expected reply to be 'World', got '%s'", reply)
			}
		}
	})
}

var msgId = new(uint32)

func BenchmarkZMQGo2Way(b *testing.B) {
	b.SetParallelism(1000)
	req := make([]zmqGo.Socket, 100)
	for i := 0; i < 100; i++ {
		req[i] = zmqGo.NewReq(context.Background())
		err := req[i].Dial("tcp://127.0.0.1:15564")
		if err != nil {
			panic(err)
		}
	}
	i := new(int32)
	b.RunParallel(func(pbt *testing.PB) {

		var (
			req       = req[atomic.AddInt32(i, 1)%100]
			toReq     = zmqGo.NewMsg([]byte("0000Hello"))
			thisMsgId uint32
		)

		for pbt.Next() {
			thisMsgId = atomic.AddUint32(msgId, 1) % 1000000
			binary.BigEndian.PutUint32(toReq.Frames[0][0:4], thisMsgId)
			//  Send request
			req.Send(toReq)

			reply := <-replyChan[thisMsgId]
			if string(reply.Frames[0][4:]) != "World" {
				b.Fatalf("Expected reply to be 'World', got '%s'", reply.Frames[0])
			}
		}
	})
}
