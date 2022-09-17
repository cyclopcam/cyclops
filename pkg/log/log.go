package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/logging"
)

type Level int

const (
	LevelDebug    Level = iota // information that only a programmer will understand
	LevelInfo                  // information that a non-programmer might be interested in
	LevelWarn                  // speeds up tracking down issues, once you know about them
	LevelError                 // should not have happened
	LevelCritical              // wake somebody up
)

type Log interface {
	Close() // Give logger a chance to flush
	Debugf(format string, a ...interface{})
	Infof(format string, a ...interface{})
	Warnf(format string, a ...interface{})
	Errorf(format string, a ...interface{})
	Criticalf(format string, a ...interface{})
}

type Logger struct {
	Output io.Writer
	GCP    *logging.Logger
	Client *logging.Client
}

type testLogWriter struct {
	t *testing.T
}

func (w *testLogWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}

func NewLog() (Log, error) {
	l := &Logger{}
	gcpProjectID := os.Getenv("GCP_PROJECT_ID")
	gcpLogname := os.Getenv("GCP_LOGNAME")
	if gcpProjectID != "" && gcpLogname != "" {
		fmt.Printf("Logging to GCP %v / %v (you won't see further logs on stdout)\n", gcpProjectID, gcpLogname)
		client, err := logging.NewClient(context.Background(), gcpProjectID)
		if err != nil {
			return nil, fmt.Errorf("Failed to create GCP logging client: %v", err)
		}
		logger := client.Logger(gcpLogname)
		l.Client = client
		l.GCP = logger
	} else {
		l.Output = os.Stdout
		l.Infof("Logging to stdout")
	}
	return l, nil
}

func NewTestingLog(t *testing.T) Log {
	output := &testLogWriter{
		t: t,
	}
	return &Logger{
		Output: output,
	}
}

func levelToGCP(level Level) logging.Severity {
	switch level {
	case LevelDebug:
		return logging.Debug
	case LevelInfo:
		return logging.Info
	case LevelWarn:
		return logging.Warning
	case LevelError:
		return logging.Error
	case LevelCritical:
		return logging.Critical
	}
	panic("Unknown log level")
}

func levelToName(level Level) string {
	switch level {
	case LevelDebug:
		return "Debug"
	case LevelInfo:
		return "Info"
	case LevelWarn:
		return "Warning"
	case LevelError:
		return "Error"
	case LevelCritical:
		return "Critical"
	}
	panic("Unknown log level")
}

func (l *Logger) write(level Level, format string, a ...interface{}) {
	if l.GCP != nil {
		l.GCP.Log(logging.Entry{
			Severity: levelToGCP(level),
			Payload:  fmt.Sprintf(format, a...),
		})
	} else {
		prefix := fmt.Sprintf("%.3f %v ", float64(time.Now().UnixNano())/1e9, levelToName(level))
		fmt.Fprintf(l.Output, prefix+format+"\n", a...)
	}
}

func (l *Logger) Close() {
	if l.GCP != nil {
		l.GCP.Flush()
		l.Client.Close()
	}
}

func (l *Logger) Debugf(format string, a ...interface{}) {
	l.write(LevelDebug, format, a...)
}

func (l *Logger) Infof(format string, a ...interface{}) {
	l.write(LevelInfo, format, a...)
}

func (l *Logger) Warnf(format string, a ...interface{}) {
	l.write(LevelWarn, format, a...)
}

func (l *Logger) Errorf(format string, a ...interface{}) {
	l.write(LevelError, format, a...)
}

func (l *Logger) Criticalf(format string, a ...interface{}) {
	l.write(LevelCritical, format, a...)
}
