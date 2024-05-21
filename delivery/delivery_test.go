package delivery

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"testing"
)

func BenchmarkNet(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			socket, err := net.Dial("tcp", "127.0.0.1:5001")
			if err != nil {
				b.Error(err)
			}
			if socket == nil {
				b.Error("nil socket")
			}
			var (
				r        = rand.Intn(100) * 100
				numBytes = make([]byte, r+4)
				buf      = make([]byte, 110000)
				l        uint32
			)
			binary.BigEndian.PutUint32(numBytes, uint32(r))
			_, err = socket.Write(numBytes)
			if err != nil {
				b.Error(err)
			}
			n, err := socket.Read(buf)
			if err == io.EOF || err == nil {
				l = binary.BigEndian.Uint32(buf[:4])
				if uint32(n) != l+4 {
					fmt.Println("return invalid", l+4, n)
				}
			} else {
				b.Error(err)
			}
			_ = socket.Close()
		}
	})
}
func BenchmarkEVIO(b *testing.B) {
	//var (
	//	block    int32 = 0
	//	blockPtr       = &block
	//)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			socket, err := net.Dial("tcp", "127.0.0.1:5000")
			if err != nil {
				b.Error(err)
			}
			if socket == nil {
				b.Error("nil socket")
			}
			var (
				r        = rand.Intn(99)*100 + 100
				numBytes = make([]byte, r+4)
				buf      = make([]byte, 110000)
				l        uint32
			)
			binary.BigEndian.PutUint32(numBytes, uint32(r))
			_, err = socket.Write(numBytes)
			if err != nil {
				b.Error(err)
			}
			//atomic.AddInt32(blockPtr, 1)
			n, err := socket.Read(buf)
			//fmt.Println("read", n, err,
			//	atomic.AddInt32(blockPtr, -1))
			if err == io.EOF || err == nil {
				l = binary.BigEndian.Uint32(buf[:4])
				if uint32(n) != l+4 {
					fmt.Println("return invalid", l+4, n)
				}
			} else {
				b.Error(err)
			}
			_ = socket.Close()
		}
	})
}

//func TestServer(t *testing.T) {
//	go InitServer()
//	time.Sleep(time.Millisecond * 100)
//	socket, err := net.Dial("tcp", "127.0.0.1:5000")
//	assert.Nil(t, err)
//	go func() {
//		var (
//			l   uint32
//			buf = make([]byte, 10240)
//		)
//		for {
//			n, err := socket.Read(buf)
//			if err == io.EOF {
//				l = binary.BigEndian.Uint32(buf[:4])
//				fmt.Println("received:", l+4, n)
//				assert.Equal(t, l+4, uint32(n))
//			} else if err != nil {
//				break
//			}
//		}
//	}()
//	var (
//		numBytes = make([]byte, 4)
//	)
//	for i := 1; i < 100; i++ {
//		binary.BigEndian.PutUint32(numBytes, uint32(i*100))
//		_, err = socket.Write(append(numBytes, bytes.Repeat([]byte("a"), i*100)...))
//		assert.Nil(t, err)
//	}
//}
