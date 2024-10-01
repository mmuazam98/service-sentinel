package logger

import (
	"fmt"
	"time"
)

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

func Success(message string) {
	logWithColor(LevelInfo, ColorGreen, message)
}

func Info(message string) {
	logWithColor(LevelInfo, ColorCyan, message)
}

func Error(message string) {
	logWithColor(LevelError, ColorRed, message)
}

func Warn(message string) {
	logWithColor(LevelWarn, ColorYellow, message)
}

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorCyan   = "\033[36m"
)

func logWithColor(level Level, color string, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("%s[%s] %s%s: %s%s\n", color, level, timestamp, ColorReset, message, ColorReset)
}
