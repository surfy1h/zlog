package report

import (
	"bufio"
	"log"
	"sync"
	"time"

	"github.com/tidwall/pretty"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

/*
Log reporting, informing of exceptions, or troubleshooting specific errors through logs
*/

type ImType string

const (
	ImTypeSlack ImType = "slack"
)

// _bufSize Give a relatively large value to avoid flush when write.
const (
	_bufSize = 1024 * 1024 //1M memory
)

type ReportConfig struct {
	//The type of report is currently supported：slack
	Type string `json:"type" mapstructure:"type"`
	//slack填token
	Token string `json:"token" mapstructure:"token"`
	//The frequency at which the log is refreshed in seconds
	FlushSec int64 `json:",default=3" mapstructure:"flushSec"`
	//The maximum number of logs is how many logs are triggered
	MaxCount int64 `json:",default=20" mapstructure:"maxCount"`
	//What level of log reporting
	Level zap.AtomicLevel `json:"Level" mapstructure:"level"`
}

func NewWriteSyncer(c ReportConfig) zapcore.WriteSyncer {
	var ws zapcore.WriteSyncer
	switch ImType(c.Type) {
	case ImTypeSlack:
		ws = NewSlackWriter(c.Token)
	default:
		log.Panicf("unsupported report type:%s", c.Type)
	}
	return ws
}

func NewReportWriterBuffer(c ReportConfig) *ReportWriterBuffer {
	ws := NewWriteSyncer(c)
	rwb := &ReportWriterBuffer{
		buf:      bufio.NewWriterSize(ws, _bufSize),
		flushSec: c.FlushSec,
		maxCount: c.MaxCount,
	}
	go rwb.Start()
	return rwb
}

type ReportWriterBuffer struct {
	buf      *bufio.Writer
	count    int64
	flushSec int64
	maxCount int64
	mu       sync.Mutex
}

func (l *ReportWriterBuffer) Start() {
	for {
		time.Sleep(time.Duration(l.flushSec) * time.Second)
		if err := l.Sync(); err != nil {
			log.Printf("report writer buffer sync error:%v", err)
		}
	}
}

// This p will be reused by zap, and it is important to note that there may be problems if the slice is not copied。
func (l *ReportWriterBuffer) Write(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	data := pretty.Pretty(p)
	if _, err := l.buf.Write(data); err != nil {
		return 0, err
	}
	l.count++
	if l.count >= l.maxCount {
		l.buf.Flush()
		l.count = 0
	}

	return len(p), nil
}

func (l *ReportWriterBuffer) Sync() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count = 0
	return l.buf.Flush()
}

func NewReporter(config ReportConfig) Reporter {
	switch config.Type {
	case string(ImTypeSlack):
		return NewSlackReporter(config)
	default:
		return NewSlackReporter(config)
	}
}

func NewSlackWriter(token string) zapcore.WriteSyncer {
	return &SlackWriter{token: token}
}

type SlackWriter struct {
	token string
}

func (w *SlackWriter) Write(p []byte) (n int, err error) {
	// Implement Slack message sending logic here
	return len(p), nil
}

func (w *SlackWriter) Sync() error {
	return nil
}
