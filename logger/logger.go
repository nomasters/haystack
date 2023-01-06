package logger

// Logger is an interface to make swapping out loggers simple
type Logger interface {
	Panicln(v ...any)
	Panicf(format string, v ...any)
	Fatalln(v ...any)
	Fatalf(format string, v ...any)
	Errorln(v ...any)
	Errorf(format string, v ...any)
	Warnln(v ...any)
	Warnf(format string, v ...any)
	Infoln(v ...any)
	Infof(format string, v ...any)
	Debugln(v ...any)
	Debugf(format string, v ...any)
	Traceln(v ...any)
	Tracf(format string, v ...any)
}
