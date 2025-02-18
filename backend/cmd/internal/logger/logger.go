package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type AppLogger interface {
	Error(err error)
	Info(msg string)
	Fatal(msg string)
	Close() error
}

type fileLogger struct {
	errorLogger *log.Logger
	infoLogger  *log.Logger
	errorFile   *os.File
	messageFile *os.File
}

const (
	errorLogFile   = "error.log"
	messageLogFile = "messages.log"
)

func New(debug bool, staticDir string) (AppLogger, error) {
	if debug {
		stdout := os.Stdout
		logger := &fileLogger{
			errorLogger: log.New(stdout, "", log.LstdFlags|log.Lshortfile),
			infoLogger:  log.New(stdout, "", log.LstdFlags|log.Lshortfile),
		}

		return logger, nil
	}

	logDir := filepath.Join(staticDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	errorFile, err := os.OpenFile(
		filepath.Join(logDir, errorLogFile),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to open error log: %w", err)
	}

	messageFile, err := os.OpenFile(
		filepath.Join(logDir, messageLogFile),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)

	if err != nil {
		errorFile.Close()
		return nil, fmt.Errorf("failed to open message log: %w", err)
	}

	logger := &fileLogger{
		errorLogger: log.New(errorFile, "", log.LstdFlags|log.Lshortfile),
		infoLogger:  log.New(messageFile, "", log.LstdFlags|log.Lshortfile),
		errorFile:   errorFile,
		messageFile: messageFile,
	}

	return logger, nil
}

func (l *fileLogger) Error(err error) {
	l.errorLogger.Printf("[ERROR] %v", err)
}

func (l *fileLogger) Info(msg string) {
	l.infoLogger.Printf("[INFO] %s", msg)
}

func (l *fileLogger) Fatal(msg string) {
	l.errorLogger.Printf("[FATAL] %s", msg)
	os.Exit(1)
}

func (l *fileLogger) Close() error {
	if l.errorFile != nil {
		err := l.errorFile.Close()
		if err != nil {
			return fmt.Errorf("failed to close error log: %w", err)
		}
	}

	if l.messageFile != nil {
		err := l.messageFile.Close()
		if err != nil {
			return fmt.Errorf("failed to close message log: %w", err)
		}
	}

	return nil
}
