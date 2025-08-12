package logger

import (
	"log"
	"os"
)

type LoggerInterface interface {
	Info(message string)
	Error(message string)
	Close() error
}

type Logger struct {
	infoLogger  *log.Logger
	errorLogger *log.Logger
	infoFile    *os.File
	errorFile   *os.File
}
