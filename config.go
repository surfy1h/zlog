package zlog

import (
	"log"
	"os"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/surfy1h/zlog/report"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// _defaultBufferSize specifies the default size used by Buffer.
	_defaultBufferSize = 256 * 1024 // 256 kB

	// _defaultFlushInterval specifies the default flush interval for
	// Buffer.
	_defaultFlushInterval = 30 * time.Second
)

const (
	_file    = "file"
	_console = "console"
)

type Config struct {
	Name string `json:",optional" mapstructure:"name"`
	// Log level: debug, info, warn, panic
	Level zap.AtomicLevel `json:"Level" mapstructure:"level"`
	// Whether to display stack trace at error level
	Stacktrace bool `json:",default=true" mapstructure:"stacktrace"`
	// Add caller information
	AddCaller bool `json:",default=true" mapstructure:"addCaller"`
	// Call stack depth for middleware that wraps logs
	CallerShip int `json:",default=3" mapstructure:"callerShip"`
	// Output mode: standard output (console) or file
	Mode string `json:",default=console" mapstructure:"mode"`
	// File name with path
	FileName string `json:",optional" mapstructure:"filename"`
	// Error level logs output location, default is console (standard error output), file can specify a file
	ErrorFileName string `json:",optional" mapstructure:"errorFileName"`
	// Log file size in MB, default is 500MB
	MaxSize int `json:",optional" mapstructure:"maxSize"`
	// Log retention days
	MaxAge int `json:",optional" mapstructure:"maxAge"`
	// Maximum number of log files to retain
	MaxBackup int `json:",optional" mapstructure:"maxBackUp"`
	// Asynchronous logging: logs are first written to memory and then periodically flushed to disk. If this is set, ensure to call Sync() when the program exits; it doesn't need to be set to true during development.
	Async bool `json:",optional" mapstructure:"async"`
	// Whether to output in JSON format
	Json bool `json:",optional" mapstructure:"json"`
	// Whether to compress logs
	Compress bool `json:",optional" mapstructure:"compress"`
	// Whether to output to console in file mode
	Console bool `json:"console" mapstructure:"console"`
	// Whether to add color in non-JSON format
	Color bool  `json:",default=true" mapstructure:"color"`
	Port  int32 `json:",default=true" mapstructure:"port"`
	// Whether to report logs
	IsReport bool `json:",optional" mapstructure:"isReport"`
	// Report configuration
	ReportConfig report.ReportConfig `json:",optional" mapstructure:"reportConfig"`
	options      []zap.Option
}

func (lc *Config) UpdateLevel(level zapcore.Level) {
	lc.Level.SetLevel(level)
}

func (lc *Config) Build() *zap.Logger {
	if lc.Mode != _file && lc.Mode != _console {
		log.Panicln("mode must be console or file")
	}

	if lc.Mode == _file && lc.FileName == "" {
		log.Panicln("file mode, but file name is empty")
	}
	var (
		ws      zapcore.WriteSyncer
		errorWs zapcore.WriteSyncer
		encoder zapcore.Encoder
	)
	encoderConfig := zapcore.EncoderConfig{
		// When the storage format is JSON, these are the keys
		MessageKey:    "msg",
		LevelKey:      "level",
		TimeKey:       "time",
		NameKey:       "logger",
		CallerKey:     "caller",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		// The output format for the above fields
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     CustomTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.FullCallerEncoder,
	}
	if lc.Mode == _console {
		ws = zapcore.Lock(os.Stdout)
	} else {
		normalConfig := &lumberjack.Logger{
			Filename:   lc.FileName,
			MaxSize:    lc.MaxSize,
			MaxAge:     lc.MaxAge,
			MaxBackups: lc.MaxBackup,
			LocalTime:  true,
			Compress:   lc.Compress,
		}
		if lc.ErrorFileName != "" {
			errorConfig := &lumberjack.Logger{
				Filename:   lc.ErrorFileName,
				MaxSize:    lc.MaxSize,
				MaxAge:     lc.MaxAge,
				MaxBackups: lc.MaxBackup,
				LocalTime:  true,
				Compress:   lc.Compress,
			}
			errorWs = zapcore.Lock(zapcore.AddSync(errorConfig))
		}
		ws = zapcore.Lock(zapcore.AddSync(normalConfig))
	}
	// Whether to add color
	if lc.Color && !lc.Json {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	encoder = zapcore.NewConsoleEncoder(encoderConfig)
	if lc.Json {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	if lc.Async {
		ws = &zapcore.BufferedWriteSyncer{
			WS:            ws,
			Size:          _defaultBufferSize,
			FlushInterval: _defaultFlushInterval,
		}
		if errorWs != nil {
			errorWs = &zapcore.BufferedWriteSyncer{
				WS:            errorWs,
				Size:          _defaultBufferSize,
				FlushInterval: _defaultFlushInterval,
			}
		}
	}

	var c = []zapcore.Core{zapcore.NewCore(encoder, ws, lc.Level)}
	if errorWs != nil {
		highCore := zapcore.NewCore(encoder, errorWs, zapcore.ErrorLevel)
		c = append(c, highCore)
	}
	// In file mode, also output to console
	if lc.Mode == _file && lc.Console {
		consoleWs := zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), zapcore.ErrorLevel)
		c = append(c, consoleWs)
	}
	if lc.IsReport {
		// Reporting format is always JSON
		if !lc.Json {
			encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		}
		// Specify the log level for reporting.
		highCore := zapcore.NewCore(encoder, report.NewReportWriterBuffer(lc.ReportConfig), lc.ReportConfig.Level)
		c = append(c, highCore)
	}

	core := zapcore.NewTee(c...)

	logger := zap.New(core)
	// Whether to add caller information
	if lc.AddCaller {
		lc.options = append(lc.options, zap.AddCaller())
		if lc.CallerShip != 0 {
			lc.options = append(lc.options, zap.AddCallerSkip(lc.CallerShip))
		}
	}
	// When an error occurs, whether to add stack information
	if lc.Stacktrace {
		// Add stack trace for error level and above
		lc.options = append(lc.options, zap.AddStacktrace(zap.PanicLevel))
	}
	if lc.Name != "" {
		logger = logger.With(zap.String("project", lc.Name))
	}
	if lc.Port > 0 {
		InitLogServer(lc.Port)
	}
	return logger.WithOptions(lc.options...)
}

func CustomTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02-15:04:05"))
}

// StringToLogLevelHookFunc: viper's string to zapcore.Level conversion
func StringToLogLevelHookFunc() mapstructure.DecodeHookFunc {
	return func(
		f reflect.Type,
		t reflect.Type,
		data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		atomicLevel, err := zap.ParseAtomicLevel(data.(string))
		if err != nil {
			return data, nil
		}
		// Convert it by parsing
		return atomicLevel, nil
	}
}
