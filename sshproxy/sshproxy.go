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
	exit := make(chan bool, 2)
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
		req_type := "shell"
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
				log.Printf("Request: %s %s %s\n", req.Type, req.WantReply, req.Payload)

				b, err := dst.SendRequest(req.Type, req.WantReply, req.Payload)
				if err != nil {
					log.Printf("some error:%s", err)
				}

				if req.WantReply {
					req.Reply(b, nil)
				}

				log.Println("request type:" + req.Type)
				req_type = req.Type

				switch req.Type {
				case "exit-status":
					break r
				case "exec":
					scp_session, err := clientConn.NewSession()
					if err != nil {
						break r
					}
					if err := scp_session.Start(string(req.Payload) + "\n"); err != nil {
						break r
					}

					<-exit
					scp_session.Wait()
					log.Println("remote scp process complete: ", string(req.Payload))
					scp_session.Close()
					break r
				case "subsystem":
					<-exit
					break r
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
			defer log.Printf("server read finish")
			defer func() { exit <- true }()
			buf := make([]byte, 128)
			for {
				size, err := wrappedChannel.Read(buf)
				if err != nil {
					break
				}
				_, ew := channel2.Write(buf[:size])
				if ew != nil {
					break
				}

				if req_type == "subsystem" {
					log.Printf("1: %x", buf[0])
				}
				// log.Printf("get msg: %s", string(buf))
			}
		}()

		go func() {
			defer log.Printf("client read finish")
			defer func() { exit <- true }()
			buf := make([]byte, 128)
			for {
				size, err := wrappedChannel2.Read(buf)
				if err != nil {
					break
				}
				_, ew := channel.Write(buf[:size])
				if ew != nil {
					break
				}
				safeMessage := base64.StdEncoding.EncodeToString([]byte(buf[:size]))
				if req_type == "shell" {
					log.Printf("post msg: %s", string(buf))
					p.cc.SendLogEvent(safeMessage, code_info.code, code_info.way)
				}
				// log.Printf("post msg: %s", string(buf))
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
