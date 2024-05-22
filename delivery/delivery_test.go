package delivery

import (
	"testing"
	"time"
)

var (
	mainServer  ILogistic
	nodeServer1 ILogistic
)

func init() {
	var (
		err error
	)
	mainServer, err = NewServer("tcp://127.0.0.1:12345", "", []string{"foo", "bar"})
	if err != nil {
		panic(err)
	}
	nodeServer1, err = NewServer("tcp://127.0.0.1:23456", "tcp://127.0.0.1:12345", []string{"foo", "bar"})
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Millisecond * 100)
}

func TestSwitch(t *testing.T) {
	nodeServer1.SwitchHost("tcp://127.0.0.1:12345")
	time.Sleep(time.Millisecond * 100)
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
