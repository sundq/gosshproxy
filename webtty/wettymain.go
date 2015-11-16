package webtty

import (
	"../libs"
	"fmt"
	"github.com/codegangsta/cli"
	"os"
	// "os/signal"
	"sync"
	// "syscall"
)

func WettyMain(wg *sync.WaitGroup, cc *libs.CenterCommunication) {
	cmd := cli.NewApp()
	cmd.Version = Version
	cmd.Name = "gotty"
	cmd.Usage = "Share your terminal as a web application"
	cmd.HideHelp = true

	flags := []flag{
		flag{"address", "a", "IP address to listen"},
		flag{"port", "p", "Port number to listen"},
		flag{"permit-write", "w", "Permit clients to write to the TTY (BE CAREFUL)"},
		flag{"credential", "c", "Credential for Basic Authentication (ex: user:pass, default disabled)"},
		flag{"random-url", "r", "Add a random string to the URL"},
		flag{"random-url-length", "", "Random URL length"},
		flag{"tls", "t", "Enable TLS/SSL"},
		flag{"tls-crt", "", "TLS/SSL certificate file path"},
		flag{"tls-key", "", "TLS/SSL key file path"},
		flag{"tls-ca-crt", "", "TLS/SSL CA certificate file for client certifications"},
		flag{"index", "", "Custom index.html file"},
		flag{"title-format", "", "Title format of browser window"},
		flag{"reconnect", "", "Enable reconnection"},
		flag{"reconnect-time", "", "Time to reconnect"},
		flag{"once", "", "Accept only one client and exit on disconnection"},
		flag{"permit-arguments", "", "Permit clients to send command line arguments in URL (e.g. http://example.com:8080/?arg=AAA&arg=BBB)"},
		flag{"close-signal", "", "Signal sent to the command process when gotty close it (default: SIGHUP)"},
	}

	mappingHint := map[string]string{
		"index":      "IndexFile",
		"tls":        "EnableTLS",
		"tls-crt":    "TLSCrtFile",
		"tls-key":    "TLSKeyFile",
		"tls-ca-crt": "TLSCACrtFile",
		"random-url": "EnableRandomUrl",
		"reconnect":  "EnableReconnect",
	}

	defer wg.Done()

	cliFlags, err := generateFlags(flags, mappingHint)
	if err != nil {
		exit(err, 3)
	}

	cmd.Flags = append(
		cliFlags,
		cli.StringFlag{
			Name:   "config",
			Value:  "~/.gotty",
			Usage:  "Config file path",
			EnvVar: "GOTTY_CONFIG",
		},
	)

	cmd.Action = func(c *cli.Context) {
		if len(c.Args()) == 0 {
			fmt.Println("Error: No command given.\n")
			cli.ShowAppHelp(c)
			exit(err, 1)
		}

		options := DefaultOptions

		configFile := c.String("config")
		_, err := os.Stat(ExpandHomeDir(configFile))
		if configFile != "~/.gotty" || !os.IsNotExist(err) {
			if err := ApplyConfigFile(&options, configFile); err != nil {
				exit(err, 2)
			}
		}

		applyFlags(&options, flags, mappingHint, c)

		if c.IsSet("credential") {
			options.EnableBasicAuth = true
		}
		if c.IsSet("tls-ca-crt") {
			options.EnableTLSClientAuth = true
		}

		if err := CheckConfig(&options); err != nil {
			exit(err, 6)
		}
		app, err := New(c.Args(), &options, cc)
		if err != nil {
			exit(err, 3)
		}

		// registerSignals(app)

		err = app.Run()
		if err != nil {
			exit(err, 4)
		}
	}

	cli.AppHelpTemplate = helpTemplate
	cmd.Run([]string{"./main", "-w", "diaobaoyun", "tty"})
}

func exit(err error, code int) {
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(code)
}

// func registerSignals(app *App) {
// 	sigChan := make(chan os.Signal, 1)
// 	signal.Notify(
// 		sigChan,
// 		syscall.SIGINT,
// 		syscall.SIGTERM,
// 	)

// 	go func() {
// 		for {
// 			s := <-sigChan
// 			switch s {
// 			case syscall.SIGINT, syscall.SIGTERM:
// 				if app.Exit() {
// 					fmt.Println("Send ^C to force exit.")
// 				} else {
// 					os.Exit(5)
// 				}
// 			}
// 		}
// 	}()
// }
