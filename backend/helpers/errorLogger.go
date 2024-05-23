package helpers

import (
	"log"
	"os"
)

func LogError(message string) {
	logFile, err := os.OpenFile("./public/errors/thumb_errors.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Printf("Failed to open error log file: %v", err)
		return
	}

	defer logFile.Close()

	errorLog := log.New(logFile, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	errorLog.Println(message)
}
