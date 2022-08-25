package tcp

import (
	"net"
	"time"
)

type ConnPair struct {
	client net.Conn
	server net.Conn
}

func prepare() (pair *ConnPair, err error) {
	network, addr := "tcp", ":8080"
	listen, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}

	type clientCon struct {
		client net.Conn
		err    error
	}

	clientChann := make(chan *clientCon)
	go func() {
		time.Sleep(1 * time.Second)
		client, err1 := net.Dial(network, addr)
		clientChann <- &clientCon{client: client, err: err1}
	}()

	pair = &ConnPair{}
	defer func() {
		if err != nil {
			if pair.server != nil {
				_ = pair.server.Close()
			}
			if pair.client != nil {
				_ = pair.client.Close()
			}
		}
	}()

	for {
		var con net.Conn
		con, err = listen.Accept()
		if err != nil {
			return
		}
		pair.server = con
		break
	}

	select {
	case cc := <-clientChann:
		if cc.err != nil {
			return nil, err
		}
		pair.client = cc.client
		return
	}
}
