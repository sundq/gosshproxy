package main

import (
	"./libs"
	"./sshproxy"
	"./webtty"
	"sync"

	"fmt"
)

func main() {
	cc, err := libs.NewCenterCommunication("/Users/sundq/workspace/pentagon/diaobaoyun.sock")
	if err != nil {
		panic("Create uinx domain sock failed")
	}
	fmt.Println("dddddd:", cc)
	var wg sync.WaitGroup
	wg.Add(2)
	go webtty.WettyMain(&wg, cc)
	go sshproxy.SshProxyMain(&wg, cc)
	wg.Wait()
}
