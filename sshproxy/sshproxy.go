package sshproxy

import (
	"../libs"
	"code.google.com/p/go.crypto/ssh"
	"encoding/base64"
	"io"
	"log"
	"net"
)

type SshConn struct {
	net.Conn
	cc         *libs.CenterCommunication
	config     *ssh.ServerConfig
	callbackFn func(c ssh.ConnMetadata) (*ssh.Client, error)
	tunnelFn   func(c ssh.ConnMetadata) (*code_info, error)
	wrapFn     func(c ssh.ConnMetadata, r io.ReadCloser) (io.ReadCloser, error)
	closeFn    func(c ssh.ConnMetadata) error
}

func (p *SshConn) serve() error {
	serverConn, chans, reqs, err := ssh.NewServerConn(p, p.config)
	if err != nil {
		log.Println("failed to handshake")
		return (err)
	}

	code_info, _ := p.tunnelFn(serverConn)
	defer serverConn.Close()
	defer p.cc.SendDisconnectEvent(code_info.code, code_info.peer_ip, code_info.way)

	p.cc.SendConnectEvent(code_info.code, code_info.peer_ip, code_info.way)

	clientConn, err := p.callbackFn(serverConn)
	if err != nil {
		log.Printf("%s", err.Error())
		return (err)
	}

	defer clientConn.Close()

	p.cc.SetDataCallback(code_info.code, code_info.way, func(data []byte) error {
		log.Printf("Get message from unix domain:", string(data))
		clientConn.Close()
		return nil
	})

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {

		channel2, requests2, err2 := clientConn.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
		if err2 != nil {
			log.Printf("Could not accept client channel: %s", err.Error())
			return err
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Printf("Could not accept server channel: %s", err.Error())
			return err
		}

		// connect requests
		go func() {
			log.Printf("Waiting for request")

		r:
			for {
				var req *ssh.Request
				var dst ssh.Channel

				select {
				case req = <-requests:
					dst = channel2
				case req = <-requests2:
					dst = channel
				}
				if req == nil {
					break r
				}
				log.Printf("Request: %s %s %s %s\n", dst, req.Type, req.WantReply, req.Payload)

				b, err := dst.SendRequest(req.Type, req.WantReply, req.Payload)
				if err != nil {
					log.Printf("%s", err)
				}

				if req.WantReply {
					req.Reply(b, nil)
				}

				log.Println(req.Type)

				switch req.Type {
				case "exit-status":
					break r
				case "exec":
					// log.Printf("exec : %s", req.Payload)
					// dst.Write(req.Payload)
					// dst.Write([]byte("\n"))
					// not supported (yet)
				default:
					log.Println(req.Type)
				}
			}

			channel.Close()
			channel2.Close()
		}()

		// connect channels
		log.Printf("Connecting channels.")

		var wrappedChannel io.ReadCloser = channel
		var wrappedChannel2 io.ReadCloser = channel2

		if p.wrapFn != nil {
			// wrappedChannel, err = p.wrapFn(channel)
			wrappedChannel2, err = p.wrapFn(serverConn, channel2)
		}

		// go io.Copy(channel2, wrappedChannel)
		// go io.Copy(channel, wrappedChannel2)

		go func() {
			buf := make([]byte, 128)
			for {
				size, err := wrappedChannel.Read(buf)
				if err != nil {
					return
				}
				channel2.Write(buf[:size])
				// log.Printf("get: %s", string(buf))
			}
		}()

		go func() {
			buf := make([]byte, 128)
			for {
				size, err := wrappedChannel2.Read(buf)
				if err != nil {
					return
				}
				channel.Write(buf[:size])
				safeMessage := base64.StdEncoding.EncodeToString([]byte(buf[:size]))
				p.cc.SendLogEvent(safeMessage, code_info.code, code_info.way)
				// log.Printf("post: %s", string(buf))
			}
		}()

		defer wrappedChannel.Close()
		defer wrappedChannel2.Close()
	}

	if p.closeFn != nil {
		p.closeFn(serverConn)
	}

	return nil
}

func ListenAndServe(addr string, serverConfig *ssh.ServerConfig, cc *libs.CenterCommunication,
	callbackFn func(c ssh.ConnMetadata) (*ssh.Client, error),
	tunnelFn func(c ssh.ConnMetadata) (*code_info, error),
	wrapFn func(c ssh.ConnMetadata, r io.ReadCloser) (io.ReadCloser, error),
	closeFn func(c ssh.ConnMetadata) error,
) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("net.Listen failed: %v", err)
		return err
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("listen.Accept failed: %v", err)
			return err
		}

		sshconn := &SshConn{Conn: conn, config: serverConfig, cc: cc, callbackFn: callbackFn, tunnelFn: tunnelFn, wrapFn: wrapFn, closeFn: closeFn}

		go func() {
			if err := sshconn.serve(); err != nil {
				log.Printf("Error occured while serving %s\n", err)
				return
			}

			log.Println("Connection closed.")
		}()
	}

}
