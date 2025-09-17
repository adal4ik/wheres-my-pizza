package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Logger struct{ service string }

func New(service string) *Logger { return &Logger{service: service} }

func (l *Logger) log(level, action, msg string, fields map[string]any, err error) {
	entry := map[string]any{
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
		"level":      level,
		"service":    l.service,
		"action":     action,
		"message":    msg,
		"hostname":   hostname(),
		"request_id": "",
	}
	if fields != nil {
		for k, v := range fields {
			entry[k] = v
		}
	}
	if err != nil {
		entry["error"] = map[string]any{"msg": err.Error(), "stack": fmt.Sprintf("%T", err)}
	}
	_ = json.NewEncoder(os.Stdout).Encode(entry)
}

func (l *Logger) Info(action string, fields map[string]any)             { l.log("INFO", action, action, fields, nil) }
func (l *Logger) Debug(action string, fields map[string]any)            { l.log("DEBUG", action, action, fields, nil) }
func (l *Logger) Error(action string, err error, fields map[string]any) { l.log("ERROR", action, action, fields, err) }

func hostname() string { h, _ := os.Hostname(); return h }
