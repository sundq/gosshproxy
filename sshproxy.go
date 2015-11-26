package main

import (
	"./libs"
	. "./log"
	"code.google.com/p/go.crypto/ssh"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"strings"
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

func BytesToInt64(buf []byte) int64 {
	return int64(binary.BigEndian.Uint64(buf))
}

func BytesToInt32(buf []byte) int {
	return int(binary.BigEndian.Uint32(buf))
}

func (p *SshConn) serve() error {
	serverConn, chans, reqs, err := ssh.NewServerConn(p, p.config)
	if err != nil {
		Log.Info("failed to handshake:%s", err)
		return (err)
	}
	exit := make(chan bool, 2)
	code_info, _ := p.tunnelFn(serverConn)
	defer serverConn.Close()
	defer p.cc.SendDisconnectEvent(code_info.code, code_info.peer_ip, code_info.way)

	p.cc.SendConnectEvent(code_info.code, code_info.peer_ip, code_info.way)

	clientConn, err := p.callbackFn(serverConn)
	if err != nil {
		Log.Info("%s", err.Error())
		return (err)
	}

	defer clientConn.Close()

	p.cc.SetDataCallback(code_info.code, code_info.way, func(data []byte) error {
		Log.Info("Get message from unix domain:", string(data))
		clientConn.Close()
		return nil
	})

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		channel2, requests2, err2 := clientConn.OpenChannel(newChannel.ChannelType(), newChannel.ExtraData())
		if err2 != nil {
			Log.Info("Could not accept client channel: %s", err.Error())
			return err
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			Log.Info("Could not accept server channel: %s", err.Error())
			return err
		}

		// connect requests
		req_type := "shell"
		req_subsystem_type := ""
		_file_op := ""
		_filename := ""
		go func() {
			Log.Info("Waiting for request")

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
				Log.Info("Request: %s %s %s\n", req.Type, req.WantReply, req.Payload)

				b, err := dst.SendRequest(req.Type, req.WantReply, req.Payload)
				if err != nil {
					Log.Info("some error:%s", err)
				}

				if req.WantReply {
					req.Reply(b, nil)
				}

				Log.Info("request type: %s", req.Type)
				req_type = req.Type
				req_subsystem_type = string(req.Payload[4:len(req.Payload)])

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

					Log.Info("req_subsystem_type:%s", req_subsystem_type)
					<-exit
					scp_session.Wait()
					Log.Info("remote scp process complete: %s %s %s", string(req.Payload), req_subsystem_type, req_type)
					scp_session.Close()
					break r
				case "subsystem":
					<-exit
					break r
				default:
					Log.Info(req.Type)
				}
			}
			channel.Close()
			channel2.Close()
		}()

		// connect channels
		Log.Info("Connecting channels.")

		var wrappedChannel io.ReadCloser = channel
		var wrappedChannel2 io.ReadCloser = channel2

		if p.wrapFn != nil {
			// wrappedChannel, err = p.wrapFn(channel)
			wrappedChannel2, err = p.wrapFn(serverConn, channel2)
		}

		// go io.Copy(channel2, wrappedChannel)
		// go io.Copy(channel, wrappedChannel2)

		go func() {
			defer Log.Info("server read finish")
			defer func() { exit <- true }()
			buf := make([]byte, 1024)
			filename := ""
			scp_data := true
			for {
				size, err := wrappedChannel.Read(buf)
				if err != nil {
					break
				}
				_, ew := channel2.Write(buf[:size])
				if ew != nil {
					break
				}
				if (req_type == "subsystem") && (req_subsystem_type == "sftp") {
					op := buf[4]
					if op == 3 { //open file
						filename_len := BytesToInt32(buf[9:13])
						if filename_len < size {
							filename = string(buf[13 : filename_len+13])
						} else {
							filename = string(buf[13:size])
						}
					}

					if op == 4 { // close file
						filename = ""
					}

					if op == 5 { //read file
						if filename != "" {
							Log.Info("download file: %s", filename)
							p.cc.SendFileLogEvent(code_info.code, filename, "download")
							filename = ""
						}
					}

					if op == 6 { // write file
						if filename != "" {
							Log.Info("upload file: %s", filename)
							p.cc.SendFileLogEvent(code_info.code, filename, "upload")
							filename = ""
						}
					}
					// Log.Info("get msg: length:%d op:%d request_id:%d", BytesToInt32(buf[:4]), buf[4], BytesToInt32(buf[5:9]), buf[:size])
				}
				if req_type == "exec" && scp_data && req_subsystem_type != "" {
					_tmp_command := strings.Split(req_subsystem_type, " ")
					if _tmp_command[1] == "-t" {
						_file_op = "upload"
						_filename = ""
					} else {
						_file_op = "download"
						_filename = _tmp_command[2]
					}
					if _file_op == "upload" {
						_filename = strings.Split(string(buf[:size]), " ")[2]
					}
					scp_data = false
					Log.Info("%s %s %s", string(buf[:size]), _filename, _file_op)
					p.cc.SendFileLogEvent(code_info.code, _filename, _file_op)
				}

			}
		}()

		go func() {
			defer Log.Info("client read finish")
			defer func() { exit <- true }()
			buf := make([]byte, 1024)
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
					// Log.Info("post msg: %s", string(buf))
					p.cc.SendLogEvent(safeMessage, code_info.code, code_info.way)
				}
				// Log.Info("post msg:", buf[:size])
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
		Log.Info("net.Listen failed: %v", err)
		return err
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			Log.Info("listen.Accept failed: %v", err)
			return err
		}

		sshconn := &SshConn{Conn: conn, config: serverConfig, cc: cc, callbackFn: callbackFn, tunnelFn: tunnelFn, wrapFn: wrapFn, closeFn: closeFn}

		go func() {
			if err := sshconn.serve(); err != nil {
				Log.Info("Error occured while serving %s\n", err)
				return
			}

			Log.Info("Connection closed.")
		}()
	}

}
