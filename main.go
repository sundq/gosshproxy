package main

import (
	"./libs"
	. "./log"
	"code.google.com/p/go.crypto/ssh"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
)

const (
	id_rsa_inner = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAtCXHKCuI3hEuY3OSuK+D8yKV4vZltg/JcGe6BVHKCeCM4ALY
gbK0mcHVhsKmM5qJBeTHxHKQp17uLx3GWVmGL8/3tJ2WNXlYupciYN+CUiTyGFjB
xdzFKt3F4Y76TmOOaT+uLuXNK4Xt6OupTDrOkAQedFPyYEkp1KCKWuDwl0eWAyFV
7+XBvQs9jm1uC0Lna9NQSzGuN1rglYd+jrRBehz0oiL1kroqdmxPXZJ/s+yhk0eC
92Bd+kPXK7QEeccg3OLbssE/XdpPny6TrMmxcAy4k9tY/lFuwSdO9T0SW4ODHj5C
imPKJme/SIl9nOLOWIgoaY8PFUj9PzqUTa2MoQIDAQABAoIBACvnWRYtJfoY7dlG
/WcNP3ct4qGhs0AfsNQ4M1nAiSDHHQ4rI2DYkLM4TjW9kovZCbPqAdWapi5kMGBD
PWfhLZbRdGkMTuNRY5J16ub5EeW7I1VTrEXwfAzqZ6OFGPOpx7dW0biUQOBuj4DI
jkYJvvXSSynGm7djnVI4nf9v+rKjUtDlo4SXrC+XucSLPScfKRbyubv72a0hmiBw
nsu34v64720pCRRDEGUjifg1mgtDMwJSiwGpJTksDApSuBly/q29g10Xd85QKbal
XZP6jEkL6bOlTYg2ZslQTyKwcoPrL3Zma5dRTS9EAvbLI+HupiSCEKFYtQTkI+VX
M4irs5ECgYEA4ykhMWAVmBynle4GKTOxf3d5LGX9352wCStlSfzZSpsNwet2NcgE
WAm1w5XSp5LvszSGmHkUun2aEf858Oy6ooCvLcUDTeeFlFD9Qep/BgUMhiwGot0D
a6agGDNzD4rBCUGgmzIHMxpxzn7zeuG/paHkqKBJT2rmQBlh8lFsm6cCgYEAywSx
JmKgg8exBDNEr2w9OS7TTSPg8o0tUllbNp2sT7BjBbzCEAodarAYln206LhWRGJi
Ep/S3n9FerOUxbdQMauR17AmHHP1lrdqYCmjdfR0QpGl5AKYlzUVys7f6wYjS5yJ
nmfczm45vEx4J9W0DSigd7evLOMbPbuw3oJjfncCgYAq49smjXPGUrK5tkVnhiEf
Zhl07pTuocFZGd54B8unMHt6f9DD+s4HKV4uXZ12kmG7vlEjfMkTJR/wAfaYrLnY
cN+ijq4/CVXJWTlYNsRhCJcCxlFCcuRwcPeNWjmBV7t22fNPGjUNyxJt4L5sFy+u
QFECHbF50z9CHwjFTeZpxQKBgQCNhX7MKFKGqoyuReLqeoSPOSIZExq7WkiusBWS
lAVLI8VTeYq6TaLF/W2WcpjK5b1VPFPrcbg4W/YeG9NopGnlxhkLhwJ6MSeQ4djo
of4NutPUl91RfbHLLjk7wEx9dDDkg4G3h+V3jYT3y0KzWhiCV8DM06Hso4AY357i
7XfX7wKBgQCO+YzEt5qzxtDEpt0zfEsatzV1TGIBNhsYBO5TAUQp/q9GFZCQRzRe
qh3h0pC8FOPswpML+bBOsyUD3q/EVxp56Ftv9DxClQuP3bICqp5TSd07lC6vikh9
IkylCc+UtScWAl7OEfOD5m7mIh6NEg3BOphPJ9rVr8yZLMqgRX5iQA==
-----END RSA PRIVATE KEY-----`
)

type code_info struct {
	code    string
	peer_ip string
	way     string
}

var (
	addr            *string = flag.String("addr", "0.0.0.0:9998", "listen address")
	loglevel        *string = flag.String("loglevel", "info", "log level")
	center_address  *string = flag.String("center_address", "/Users/sundq/workspace/pentagon/diaobaoyun.sock", "notify center address")
	diaobaoyun_host *string = flag.String("diaobaoyun_host", "www.diaobaoyun.com", "hostname of diaobao.")
	diaobaoyun_ssl  *string = flag.String("diaobaoyun_ssl", "https", "is https")
)

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
	flag.Parse()
	db_config, err := libs.GetConfig("./config-ssh.yaml")
	if err != nil {
		port, _ := strconv.Atoi(strings.Split(*addr, ":")[1])
		n_config := libs.Configure{
			Port:           port,
			Level:          *loglevel,
			CenterAddress:  *center_address,
			DiaobaoYunHost: *diaobaoyun_host,
			DiaobaoYunSsl:  *diaobaoyun_ssl == "https",
		}
		db_config = &n_config
		libs.SetConfig("./config-ssh.yaml", &n_config)
	}
	LogInit()
	listen := fmt.Sprintf(":%d", db_config.Port)
	key := "./id_rsa"
	cc, err := libs.NewCenterCommunication(db_config.CenterAddress)
	if err != nil {
		Log.Error("Create uinx domain sock failed")
		return
	}

	privateBytes, err := ioutil.ReadFile(key)
	if err != nil {
		// panic("Failed to load private key")
		privateBytes = []byte(id_rsa_inner)
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
	fmt.Printf("server listen %s\n", listen)
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
