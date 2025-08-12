package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func New(debug bool, logsDir string) (LoggerInterface, error) {
	var logger Logger

	if debug {
		logger.infoLogger = log.New(os.Stdout, "[INFO] ", log.LstdFlags)
		logger.errorLogger = log.New(os.Stderr, "[ERROR] ", log.LstdFlags)

		return &logger, nil
	}

	err := os.MkdirAll(logsDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	infoLogPath := filepath.Join(logsDir, "igloo_info.log")

	infoFile, err := os.OpenFile(infoLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open info log file: %w", err)
	}

	errorLogPath := filepath.Join(logsDir, "igloo_error.log")

	errorFile, err := os.OpenFile(errorLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		infoFile.Close()
		return nil, fmt.Errorf("failed to open error log file: %w", err)
	}

	logger.infoFile = infoFile
	logger.errorFile = errorFile
	logger.infoLogger = log.New(infoFile, "[INFO] ", log.LstdFlags)
	logger.errorLogger = log.New(errorFile, "[ERROR] ", log.LstdFlags)

	return &logger, nil
}

func (l *Logger) Info(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedMessage := fmt.Sprintf("[%s] %s", timestamp, message)
	l.infoLogger.Printf("%s", formattedMessage)
}

func (l *Logger) Error(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedMessage := fmt.Sprintf("[%s] %s", timestamp, message)
	l.errorLogger.Printf("%s", formattedMessage)
}

func (l *Logger) Close() error {
	var errs []error

	if l.infoFile != nil {
		if err := l.infoFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close info log file: %w", err))
		}
	}

	if l.errorFile != nil {
		if err := l.errorFile.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close error log file: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing logger: %v", errs)
	}

	return nil
}
