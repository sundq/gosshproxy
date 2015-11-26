package main

import (
	"./libs"
	. "./log"
	"code.google.com/p/go.crypto/ssh"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"
	// "sync"
)

type code_info struct {
	code    string
	peer_ip string
	way     string
}

func get_user_name(user string) (string, string, int, string, error) {
	tmp_user := strings.Split(user, "_")
	if len(tmp_user) < 2 {
		return "", "", 0, "", errors.New("User does not exist")
	}

	code_info, err := libs.GetCodeInfo(tmp_user[0])
	if err != nil {
		return "", "", 0, "", err
	}

	target_user := strings.Join(tmp_user[1:len(tmp_user)], "_")
	target_ip, _ := code_info.Get("resource").Get("ip_address").String()
	target_port, _ := code_info.Get("port").Int()

	return target_user, target_ip, target_port, tmp_user[0], nil
}

func main() {
	db_config, err := libs.GetConfig()
	listen := fmt.Sprintf(":%d", db_config.SshPort)
	key := db_config.SshPrivateKey
	cc, err := libs.NewCenterCommunication("/Users/sundq/workspace/pentagon/diaobaoyun.sock")
	if err != nil {
		Log.Error("Create uinx domain sock failed")
		return
	}

	privateBytes, err := ioutil.ReadFile(key)
	if err != nil {
		panic("Failed to load private key")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		panic("Failed to parse private key")
	}

	var sessions map[net.Addr]map[string]interface{} = make(map[net.Addr]map[string]interface{})

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			fmt.Printf("Login attempt: %s, user %s", c.RemoteAddr(), c.User())

			sessions[c.RemoteAddr()] = map[string]interface{}{
				"username": c.User(),
				"password": string(pass),
			}
			user, ip, port, code, err := get_user_name(c.User())
			if err != nil {
				return nil, err
			}
			clientConfig := &ssh.ClientConfig{}
			clientConfig.User = user
			clientConfig.Auth = []ssh.AuthMethod{
				// ssh.PublicKeys(signer),
				ssh.Password(string(pass)),
			}
			Log.Info("connect to dest:", fmt.Sprintf("%s:%d", ip, port), err)
			client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), clientConfig)
			sessions[c.RemoteAddr()]["client"] = client
			sessions[c.RemoteAddr()]["code"] = &code_info{
				code:    code,
				way:     "terminal",
				peer_ip: c.RemoteAddr().String(),
			}
			return nil, err
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			fmt.Printf("Login attempt: %s, user %s", c.RemoteAddr(), c.User())

			sessions[c.RemoteAddr()] = map[string]interface{}{
				"username": c.User(),
				"password": string(key.Marshal()),
			}
			user, ip, port, code, err := get_user_name(c.User())
			if err != nil {
				return nil, err
			}
			clientConfig := &ssh.ClientConfig{}
			clientConfig.User = user
			signer, _ := ssh.ParsePrivateKey(privateBytes)
			clientConfig.Auth = []ssh.AuthMethod{
				ssh.PublicKeys(signer),
				// ssh.Password(string(pass)),
			}
			Log.Info("connect to dest:", fmt.Sprintf("%s:%d", ip, port), err)
			client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), clientConfig)
			sessions[c.RemoteAddr()]["client"] = client
			sessions[c.RemoteAddr()]["code"] = &code_info{
				code:    code,
				way:     "terminal",
				peer_ip: c.RemoteAddr().String(),
			}
			return nil, err
		},
	}

	config.AddHostKey(private)

	ListenAndServe(listen, config, cc, func(c ssh.ConnMetadata) (*ssh.Client, error) {
		meta, _ := sessions[c.RemoteAddr()]
		client := meta["client"].(*ssh.Client)
		Log.Info("Connection accepted from: %s", c.RemoteAddr())

		return client, err
	}, func(c ssh.ConnMetadata) (*code_info, error) {
		meta, _ := sessions[c.RemoteAddr()]
		code_info := meta["code"].(*code_info)
		Log.Info("code info: %s", code_info)
		return code_info, nil
	}, func(c ssh.ConnMetadata, r io.ReadCloser) (io.ReadCloser, error) {
		return NewTypeWriterReadCloser(r), nil
	}, func(c ssh.ConnMetadata) error {
		Log.Info("Connection closed.")
		return nil
	})
}
