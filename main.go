package main

import (
	"./libs"
	"./sshproxy"
	"./webtty"
	"log"
	"sync"
)

func main() {
	InitMain()
	cc, err := libs.NewCenterCommunication("./diaobaoyun.sock")
	if err != nil {
		log.Printf("Create uinx domain sock failed")
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go webtty.WettyMain(&wg, cc)
	go sshproxy.SshProxyMain(&wg, cc)
	wg.Wait()
}
