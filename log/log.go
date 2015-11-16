package log

import (
	l4g "github.com/alecthomas/log4go"
)

var Log l4g.Logger

func init() {
	if Log == nil {
		Log = make(l4g.Logger)
		flw := l4g.NewFileLogWriter("diaobaoyun.log", true)
		flw.SetFormat("[%D %T] [%L] (%S) %M")
		flw.SetRotate(true)
		flw.SetRotateSize(1024 * 1024)
		flw.SetRotateLines(1024 * 1024)
		flw.SetRotateDaily(true)
		Log.AddFilter("file", l4g.INFO, flw)
	}
}
