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
日志上报，告知异常，具体错误还是通过日志排查
*/

type ImType string

const (
	ImTypeSlack ImType = "slack"
)

// _bufSize 给一个比较大的值，避免write的时候出现flush的情况。
const (
	_bufSize = 1024 * 1024 //1M 内存
)

type ReportConfig struct {
	//上报的类型，目前支持：slack
	Type string `json:"type" mapstructure:"type"`
	//slack填token
	Token string `json:"token" mapstructure:"token"`
	//日志刷新的频率 单位秒
	FlushSec int64 `json:",default=3" mapstructure:"flushSec"`
	//最大日志数量即达到多少条会触发刷新
	MaxCount int64 `json:",default=20" mapstructure:"maxCount"`
	//什么级别的日志上报
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

// 这个p会被zap复用，一定要注意,如果不拷贝该切片可能会出现问题。
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
