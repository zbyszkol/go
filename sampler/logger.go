package sampler

import (
	"log"
	"os"
)

var Logger ILogger

type ILogger interface {
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
	ErrorPrint(v ...interface{})
	ErrorPrintf(format string, v ...interface{})
	ErrorPrintln(v ...interface{})
}

type wrappedLogger struct {
	standard *log.Logger
	error    *log.Logger
}

func (logger *wrappedLogger) Printf(format string, v ...interface{}) {
	logger.standard.Printf(format, v)
}

func (logger *wrappedLogger) Println(v ...interface{}) {
	logger.standard.Println(v)
}

func (logger *wrappedLogger) ErrorPrintf(format string, v ...interface{}) {
	logger.error.Printf(format, v)
}

func (logger *wrappedLogger) ErrorPrintln(v ...interface{}) {
	logger.error.Println(v)
}

func (logger *wrappedLogger) Print(v ...interface{}) {
	logger.standard.Print(v)
}

func (logger *wrappedLogger) ErrorPrint(v ...interface{}) {
	logger.error.Print(v)
}

func init() {
	Logger = &wrappedLogger{standard: log.New(os.Stdout, "", log.Lshortfile), error: log.New(os.Stderr, "", log.Lshortfile)}
}
