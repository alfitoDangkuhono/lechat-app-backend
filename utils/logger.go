package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

type Logger struct {
	writer io.Writer
}

func NewLogger() *Logger {
	os.MkdirAll("logs", os.ModePerm)

	writer, err := rotatelogs.New(
		"logs/log-%y-%m-%d.log",
		rotatelogs.WithRotationTime(24*time.Hour),
		rotatelogs.WithMaxAge(30*24*time.Hour),
	)

	if err != nil {
		panic("failed to create file logs" + err.Error())
	}

	gin.DefaultWriter = writer
	gin.DefaultErrorWriter = writer

	return &Logger{writer: writer}
}

func (l *Logger) log(level, msg interface{}) {

	var message string

	switch v := msg.(type) {
	case error:
		message = v.Error()
	case string:
		message = v
	default:
		b, err := json.MarshalIndent(v, "", " ")

		if err != nil {
			message = fmt.Sprintf("%v", v)
		} else {
			message = string(b)
		}
	}
	line := fmt.Sprintf("%s | %-5s | %s\n", time.Now().Format(time.RFC3339), level, message)

	l.writer.Write([]byte(line))
}

func (l *Logger) Writer() io.Writer {
	return l.writer
}

func (l *Logger) Info(label string, data interface{}) {
	l.log("INFO "+label, data)
}
func (l *Logger) Warn(msg interface{}) {
	l.log("WARN", msg)
}
func (l *Logger) Error(msg interface{}) {
	l.log("ERROR", msg)
}
func (l *Logger) Debug(msg interface{}) {
	l.log("DEBUG", msg)
}
func (l *Logger) Message(label string, data interface{}) {
	l.log("message: ", label)
}
