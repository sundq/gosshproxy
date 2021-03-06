package libs

import (
	"errors"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/parnurzeal/gorequest"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"time"
)

type Configure struct {
	Port           int    `yaml:"port"`
	Level          string `yaml:"level"`
	DiaobaoYunHost string `yaml:"diaobaoyun_host"`
	DiaobaoYunSsl  bool   `yaml:"diaobao_ssl"`
	CenterAddress  string `yaml:"center_address"`
}

var config *Configure

// func init() {
// 	contents, _ := ioutil.ReadFile("./config.yaml")
// 	if config == nil {
// 		config = new(Configure)
// 		yaml.Unmarshal(contents, config)
// 	}
// }

func GetConfig(config_path string) (*Configure, error) {
	contents, err := ioutil.ReadFile(config_path)
	if err != nil {
		return nil, err
	} else {
		config = new(Configure)
		yaml.Unmarshal(contents, config)
		return config, nil
	}
}

func SetConfig(config_path string, n_config *Configure) {
	c, _ := yaml.Marshal(&n_config)
	config = n_config
	ioutil.WriteFile(config_path, c, 0644)
}

func GetCodeInfo(code string) (*simplejson.Json, error) {
	var scheme string
	if config.DiaobaoYunSsl {
		scheme = "https"
	} else {
		scheme = "http"
	}

	url := fmt.Sprintf("%s://%s/api/get_tunnel_detail?code=%s&token=333d9987c1b560", scheme, config.DiaobaoYunHost, code)
	request := gorequest.New()
	resp, body, errs := request.Get(url).End()
	if len(errs) > 0 {
		return nil, errs[0]
	}

	js, err := simplejson.NewJson([]byte(body))
	if resp.StatusCode == 200 {
		return js, err
	} else {
		if err != nil && js != nil {
			msg := js.Get("error_message")
			err_msg, _ := msg.String()
			return nil, errors.New(err_msg)
		} else {
			return nil, errors.New("tunnel not found")
		}
	}

}

type CenterCommunication struct {
	dataCallback         map[string]func([]byte) error //[]func([]byte) error
	unixDomainConnection net.Conn
}

func NewCenterCommunication(path string) (*CenterCommunication, error) {
	for {
		conn, err := net.Dial("unix", path)
		if err != nil {
			fmt.Printf("create connect to diaobaoyun faile: %s", err)
			// Log.Info("create connect to diaobaoyun faile: %s", err)
			time.Sleep(time.Second * 1)
			continue
		}
		cc := &CenterCommunication{unixDomainConnection: conn, dataCallback: make(map[string]func([]byte) error)}
		go func() {
			buff := make([]byte, 128)
			for {
				size, err := cc.unixDomainConnection.Read(buff)
				// Log.Info("get message from unix sock:", string(buff))
				if err != nil {
					return
				}
				js, err := simplejson.NewJson(buff[:size])
				if err == nil {
					code, err := js.Get("code").String()
					// Log.Info("disconnect code:", code, cc.dataCallback)
					if err == nil {
						cb, ok := cc.dataCallback[code+"wetty"]
						if ok {
							cb(buff)
						} else {
							cb, ok := cc.dataCallback[code+"terminal"]
							if ok {
								cb(buff)
							} else {
								// Log.Info("callback not found")
							}
						}
					}
					delete(cc.dataCallback, code+"terminal")
					delete(cc.dataCallback, code+"wetty")
				}
			}
		}()
		return cc, nil
	}
}

func (cc *CenterCommunication) SendConnectEvent(code string, peer_ip string, way string) (n int, err error) {
	js := simplejson.New()
	js.Set("type", "open_connect")
	js.Set("code", code)
	js.Set("way", way)
	js.Set("peer_ip", peer_ip)

	t, _ := js.Encode()
	return cc.unixDomainConnection.Write(t)
}

func (cc *CenterCommunication) SendDisconnectEvent(code string, peer_ip string, way string) (n int, err error) {
	js := simplejson.New()
	js.Set("type", "close_connect")
	js.Set("code", code)
	js.Set("way", way)
	js.Set("peer_ip", peer_ip)

	t, _ := js.Encode()
	delete(cc.dataCallback, code+way)
	return cc.unixDomainConnection.Write(t)
}

func (cc *CenterCommunication) SendLogEvent(data string, code string, way string) (n int, err error) {
	js := simplejson.New()
	js.Set("type", "log")
	js.Set("code", code)
	js.Set("way", way)
	js.Set("data", data)

	t, _ := js.Encode()
	return cc.unixDomainConnection.Write(t)
}

func (cc *CenterCommunication) SendFileLogEvent(code string, filename string, op string) (n int, err error) {
	sub_data := simplejson.New()
	sub_data.Set("op", op)
	sub_data.Set("filename", filename)

	js := simplejson.New()
	js.Set("type", "file_log")
	js.Set("code", code)
	js.Set("way", "terminal")
	js.Set("data", sub_data)

	t, _ := js.Encode()
	return cc.unixDomainConnection.Write(t)
}

func (cc *CenterCommunication) SetDataCallback(code string, way string, callbackFn func([]byte) error) error {
	cc.dataCallback[code+way] = callbackFn
	return nil
}
