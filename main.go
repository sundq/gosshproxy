package main

import (
	"./libs"
	"./sshproxy"
	"./webtty"
	"github.com/codegangsta/cli"
	"log"
	"os"
	"sync"
)

func main() {
	cmd := cli.NewApp()
	// cmd.Version = Version
	cmd.Usage = "Share your terminal as a web application"
	cmd.HideHelp = true

	flags := []cli.Flag{
		cli.StringFlag{Name: "key", Value: "", Usage: "agent key"},
		cli.StringFlag{Name: "hostname", Value: "", Usage: "hostname of agent"},
	}
	cmd.Flags = flags

	cmd.Action = func(c *cli.Context) {
		libs.SetConfig(c.String("key"), c.String("hostname"))
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

	cmd.Run(os.Args)
}
