package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	stdout    *log.Logger
	file      *os.File
	logOut    *log.Logger
	mu        sync.Mutex
	logFile   string
	debugMode bool
)

func Init() {
	logFile = "s3server.log"
	debugMode = false

	stdout = log.New(os.Stdout, "", 0)

	var err error
	file, err = os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		stdout.Fatalf("Failed to open log file: %v", err)
	}

	logOut = log.New(file, "", log.Ldate|log.Ltime|log.Lshortfile)
}

func SetDebug(enabled bool) {
	debugMode = enabled
}

func Debug(format string, v ...interface{}) {
	if !debugMode {
		return
	}
	writeLog("DEBUG", fmt.Sprintf(format, v...))
}

func Info(format string, v ...interface{}) {
	writeLog("INFO", fmt.Sprintf(format, v...))
}

func Error(format string, v ...interface{}) {
	writeLog("ERROR", fmt.Sprintf(format, v...))
}

func Warning(format string, v ...interface{}) {
	writeLog("WARNING", fmt.Sprintf(format, v...))
}

func writeLog(level, msg string) {
	mu.Lock()
	defer mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	_, filename := filepath.Split(os.Args[0])

	logOut.Printf("[%s] [%s] [%s] %s", timestamp, level, filename, msg)
}

func Exception(err error, context string) {
	if err == nil {
		return
	}
	Error("%s: %v", context, err)
}
