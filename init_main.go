package main

import (
	"./libs"
	. "./log"
	"fmt"
	"github.com/bitly/go-simplejson"
	"golang.org/x/net/websocket"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	HeartBeatRespTimeout = 5
	HeartBeatInterval    = 30
)

var g_conn_websocket *websocket.Conn
var g_conn_unix []net.Conn

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func hosts(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}
	// remove network address and broadcast address
	return ips[:len(ips)-1], nil
}

func ScanPort(cidr string, port int, cb func(ip string, port int, state string)) {
	Log.Info("scan port: %s:%d", cidr, port)
	ipaddresses, err := hosts(cidr)
	if err != nil {
		return
	}

	for _, ip := range ipaddresses {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, port), time.Millisecond*20)
		if err != nil {
			cb(ip, port, "close")
		} else {
			conn.Close()
			cb(ip, port, "open")
		}
	}
}

func handle_uninx_connection(conn net.Conn) {
	buff := make([]byte, 1024)
	for {
		size, err := conn.Read(buff)
		if err != nil {
			Log.Error("read data from unix domain failed: %s", err)
			break
		} else {
			Log.Info("get unix mesg: %s", buff[:size])
		}
		// g_conn_websocket.Write(buff[:size])
		SendMessage(string(buff[:size]))
	}
}

func CreateUnixDomainServer() error {
	os.Remove("./diaobaoyun.sock")
	ln, err := net.Listen("unix", "./diaobaoyun.sock")
	if err != nil {
		Log.Error("create unix domain server failed:", err)
		return err
	} else {
		Log.Info("create unix domain ok")
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				Log.Error("accept unix conn failed: %s", err)
			} else {
				Log.Info("unix connn accept.")
			}
			g_conn_unix = append(g_conn_unix, conn)
			go handle_uninx_connection(conn)
		}
	}()

	return nil

}

func SendMessage(msg string) {
	contents := fmt.Sprintf("%010d%s", len(msg), string(msg))
	if g_conn_websocket != nil {
		g_conn_websocket.Write([]byte(contents))
	}
}
func CreateConnection() (*websocket.Conn, error) {
	config, _ := libs.GetConfig()
	url := ""
	if config.DiaobaoYunSsl {
		url = fmt.Sprintf("wss://%s/api/ws/data/event", config.DiaobaoYunHost)
	} else {
		url = fmt.Sprintf("ws://%s/api/ws/data/event", config.DiaobaoYunHost)
	}

	conn, err := websocket.Dial(url, "dataEvent", "http://localhost/")
	if err == nil {
		g_conn_websocket = conn
		js := simplejson.New()
		js.Set("type", "agent_online")
		js.Set("version", config.Version)
		js.Set("web_port", config.WettyPort)
		js.Set("rdp_port", config.WebRdpPort)
		js.Set("ssh_port", config.SshPort)
		js.Set("key", config.Key)
		t, _ := js.Encode()
		SendMessage(string(t))
	}

	return conn, err
}

func HealthCheck(conn *websocket.Conn) {
	send_heartbeat_chan := make(chan bool, 1)
	defer close(send_heartbeat_chan)
	config, _ := libs.GetConfig()
	js := simplejson.New()
	js.Set("type", "health_check")
	js.Set("version", config.Version)
	js.Set("web_port", config.WettyPort)
	js.Set("rdp_port", config.WebRdpPort)
	js.Set("ssh_port", config.SshPort)
	js.Set("key", config.Key)
	t, err := js.Encode()
	if err != nil {
		Log.Info("json encode failed: %s", err)
		return
	}
	var heartbeat_timer *time.Timer
	go func() {
		for {
			SendMessage(string(t))
			Log.Info("send heartbeat...")
			heartbeat_timer = time.AfterFunc(time.Second*HeartBeatRespTimeout, func() {
				Log.Warn("heartbeat timer expired")
				_, err := CreateConnection()
				if err != nil {
					Log.Error("reconnect failed:", err)
				} else {
					send_heartbeat_chan <- true
				}
			})
			time.Sleep(time.Second * HeartBeatInterval)
			<-send_heartbeat_chan
		}
	}()

	buff := make([]byte, 512)
	for {
		Log.Info("start read data from ws")
		size, err := g_conn_websocket.Read(buff)
		if err != nil {
			Log.Info("read data failed: %s", err)
			time.Sleep(time.Second * 1)
			// send_heartbeat_chan <- true
			continue
		}
		Log.Info("get ws msg: %s", buff[:size])
		js, err := simplejson.NewJson([]byte(buff[:size]))
		if err != nil {
			Log.Info("parse data failed: %s", err)
			time.Sleep(time.Second * HeartBeatInterval)
			continue
		}
		_type, _ := js.Get("type").String()
		if _type == "health_check" {
			Log.Info("get heartbeat resp")
			send_heartbeat_chan <- true
			heartbeat_timer.Stop()
		} else if _type == "service_discovery" {
			cidr, _ := js.Get("cidr").String()
			port_s, _ := js.Get("ports").GetIndex(0).String()
			port, _ := strconv.Atoi(port_s)
			task_id, _ := js.Get("task_id").String()
			ScanPort(cidr, port, func(ip string, port int, status string) {
				t_js := simplejson.New()
				t_js.Set("type", "service_discovery")
				t_js.Set("task_id", task_id)
				r_js := simplejson.New()
				r_js.Set("ip", ip)
				r_js.Set("port", port)
				r_js.Set("status", status)
				t_js.Set("result", r_js)
				result, _ := t_js.Encode()
				SendMessage(string(result))
			})
		} else if _type == "test_port" {
			host, _ := js.Get("host").String()
			port, _ := js.Get("port").Int()
			task_id, _ := js.Get("task_id").String()
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Millisecond*20)
			t1_js := simplejson.New()
			t1_js.Set("type", "test_port")
			t1_js.Set("task_id", task_id)
			// type: 'test_port', result: ok, task_id: data.task_id
			if err != nil {
				t1_js.Set("result", "True")
				result, _ := t1_js.Encode()
				SendMessage(string(result))
			} else {
				t1_js.Set("result", "False")
				result, _ := t1_js.Encode()
				SendMessage(string(result))
				conn.Close()
			}
		} else {
			for _, conn := range g_conn_unix {
				conn.Write(buff[:size])
			}
		}

	}
}

func InitMain() {
	CreateUnixDomainServer()
	conn, err := CreateConnection()
	if err != nil {
		Log.Error("connect failed: %s", err)
		return
	}
	go HealthCheck(conn)
}
