package libs

import (
	"errors"
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/parnurzeal/gorequest"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net"
)

type Configure struct {
	SshPort        int    `yaml:"SshPort"`
	WettyPort      int    `yaml:"WettyPort"`
	WebRdpPort     int    `yaml:"WebRdpPort"`
	LogPath        string `yaml:"LogPath"`
	LogLevel       string `yaml:"LogLevel"`
	DiaobaoYunHost string `yaml:"DiaobaoYunHost"`
	DiaobaoYunSsl  bool   `yaml:"DiaobaoYunSsl"`
	Hostname       string `yaml:"Hostname"`
	Key            string `yaml:"Key"`
	Version        string `yaml:"Version"`
}

var config *Configure

func GetConfig() (*Configure, error) {
	contents, err := ioutil.ReadFile("./config.yaml")
	if err != nil {
		return nil, err
	}

	if config == nil {
		config = new(Configure)
		yaml.Unmarshal(contents, config)
	}
	return config, nil
}

func SetConfig(key string, hostname string) {
	contents, _ := ioutil.ReadFile("./config.yaml")
	config_default := Configure{}
	yaml.Unmarshal(contents, &config_default)

	if hostname != "" {
		config_default.Hostname = hostname
	}

	if key != "" {
		config_default.Key = key
	}

	c, _ := yaml.Marshal(&config_default)

	ioutil.WriteFile("./config.yaml", c, 0644)

}

func GetCodeInfo(code string) (*simplejson.Json, error) {
	GetConfig()
	url := fmt.Sprintf("http://%s/api/get_tunnel_detail?code=%s&token=333d9987c1b560", config.DiaobaoYunHost, code)
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
	conn, err := net.Dial("unix", path)
	if err == nil {
		cc := &CenterCommunication{unixDomainConnection: conn, dataCallback: make(map[string]func([]byte) error)}
		go func() {
			buff := make([]byte, 128)
			for {
				size, err := cc.unixDomainConnection.Read(buff)
				log.Printf("get message from unix sock:", string(buff))
				if err != nil {
					return
				}
				js, err := simplejson.NewJson(buff[:size])
				if err == nil {
					code, err := js.Get("code").String()
					log.Printf("disconnect code:", code, cc.dataCallback)
					if err == nil {
						cb, ok := cc.dataCallback[code+"wetty"]
						if ok {
							cb(buff)
						} else {
							cb, ok := cc.dataCallback[code+"terminal"]
							if ok {
								cb(buff)
							} else {
								log.Printf("callback not found")
							}
						}
					}
					delete(cc.dataCallback, code+"terminal")
					delete(cc.dataCallback, code+"wetty")
				}
			}
		}()
		return cc, nil
	} else {
		log.Printf("Create cc failed:", err)
		return nil, err
	}
}

func (cc *CenterCommunication) SendConnectEvent(code string, peer_ip string, way string) (n int, err error) {
	// JSON.stringify(type: "open_connect", code: _code, way: "wetty", peer_ip: _remote_ip)
	js := simplejson.New()
	js.Set("type", "open_connect")
	js.Set("code", code)
	js.Set("way", way)
	js.Set("peer_ip", peer_ip)

	t, _ := js.Encode()
	return cc.unixDomainConnection.Write(t)
}

func (cc *CenterCommunication) SendDisconnectEvent(code string, peer_ip string, way string) (n int, err error) {
	// JSON.stringify(type: "open_connect", code: _code, way: "wetty", peer_ip: _remote_ip)
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
	//JSON.stringify(type: 'log', code: _code, way: 'wetty', data: data)
	js := simplejson.New()
	js.Set("type", "log")
	js.Set("code", code)
	js.Set("way", way)
	js.Set("data", data)

	t, _ := js.Encode()
	return cc.unixDomainConnection.Write(t)
	// fmt.Fprintf(cc.unixDomainConnection, string(t))
}

func (cc *CenterCommunication) SetDataCallback(code string, way string, callbackFn func([]byte) error) error {
	//JSON.stringify(type: 'log', code: _code, way: 'wetty', data: data)
	cc.dataCallback[code+way] = callbackFn
	return nil
}
