package log

import (
	"../libs"
	l4g "github.com/alecthomas/log4go"
)

var Log l4g.Logger

func LogInit() {
	config, _ := libs.GetConfig("./config-ssh.yaml")
	level := make(map[string]l4g.Level)

	level["FINEST"] = l4g.FINEST
	level["FINE"] = l4g.FINE
	level["DEBUG"] = l4g.DEBUG
	level["TRACE"] = l4g.TRACE
	level["INFO"] = l4g.INFO
	level["WARN"] = l4g.WARNING
	level["ERROR"] = l4g.ERROR
	level["CRITICAL"] = l4g.CRITICAL

	if Log == nil {
		Log = make(l4g.Logger)
		flw := l4g.NewFileLogWriter("./ssh_proxy.log", true)
		flw.SetFormat("[%D %T] [%L] (%S) %M")
		flw.SetRotate(true)
		flw.SetRotateSize(1024 * 1024)
		flw.SetRotateLines(1024 * 1024)
		flw.SetRotateDaily(true)
		Log.AddFilter("file", level[config.Level], flw)
	}
}
