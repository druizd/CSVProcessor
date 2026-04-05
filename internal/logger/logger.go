package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
	EventLogger *log.Logger
	logFile     *os.File
)

func InitLogger(logsDir string) error {
	today := time.Now().Format("2006-01-02")
	logPath := filepath.Join(logsDir, fmt.Sprintf("service_%s.log", today))

	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	logFile = file

	InfoLogger = log.New(file, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(file, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
	EventLogger = log.New(file, "EVENT: ", log.Ldate|log.Ltime|log.Lshortfile)

	// Also print to console
	log.SetOutput(os.Stdout)
	return nil
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

func Info(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Printf("INFO: %s", msg)
	if InfoLogger != nil {
		InfoLogger.Println(msg)
	}
}

func Error(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Printf("ERROR: %s", msg)
	if ErrorLogger != nil {
		ErrorLogger.Println(msg)
	}
}

func Event(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Printf("EVENT: %s", msg)
	if EventLogger != nil {
		EventLogger.Println(msg)
	}
}
