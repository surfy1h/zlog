# zaplog: Report logs to IM tools like Feishu and WeChat Work

A simple wrapper for zap. Full address: https://github.com/luxun9527/zaplog. If you find it helpful, your star is my motivation to update.

During development, it can be used out of the box. Through this library, you can understand the usage of various common configurations of zap. It supports reporting logs above a specified level to IM tools like Feishu, WeChat Work, and Telegram.

```yaml
Name: test-project # Optional. If filled, adds a {"project": "Name"} field.
Level: info  # Log level: debug, info, warn, error
Stacktrace: true # Default is true. Shows stack trace at error level and above.
AddCaller: true # Default is true. Adds caller information.
CallShip: 3 # Default is 3. Call stack depth.
Mode: console # Default is console. Output to console or file.
Json: false # Default is false. Whether to format as JSON.
FileName:  # Optional. File mode parameter. Output to specified file.
ErrorFileName:  # Optional. File mode parameter. Where to output error logs.
MaxSize: 0 # Optional. File mode parameter. File size limit in MB.
MaxAge: 0 # Optional. File mode parameter. Maximum file retention time in days.
MaxBackup: 0 # Optional. File mode parameter. Maximum number of log files.
Async: false # Default is false. File mode parameter. Whether to write asynchronously.
Compress: false # Default is false. File mode parameter. Whether to compress.
Console: false # Default is false. File mode parameter. Whether to output to console simultaneously.
Color: true # Default is false. Whether to output in color. Recommended to use during development.
IsReport: true  # Default is false. Whether to report to IM tools. If reporting is enabled, you need to call sync at the end of the program.
ReportConfig: # Reporting configuration. Reports to IM tools at warn level and above.
  Type: lark # Optional. lark (Feishu is also this), wx, tg.
  Token: https://open.feishu.cn/open-apis/bot/v2/hook/71f86ea61212-ab9a23-464512-b40b-1be001212ffe910a # For lark, fill in the group bot webhook. For tg, fill in the token. For wx, fill in the key. This example address is invalid.
  ChatID: 0 # Fill in chatID for tg. Others do not need to be filled.
  FlushSec: 3 # Refresh interval in seconds. Set smaller for development testing, larger for production.
  MaxCount: 20 # Maximum cache count. Triggers sending when reaching refresh interval or maximum record count. Set smaller for development testing, larger for production.
  Level: warn # Specify reporting level.

```



```go
package zaplog

import (
	"github.com/luxun9527/zaplog/report"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"log"
	"os"
	"reflect"
	"time"
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
	//日志级别 debug info warn panic
	Level zap.AtomicLevel `json:"Level" mapstructure:"level"`
	//在error级别的时候 是否显示堆栈
	Stacktrace bool `json:",default=true" mapstructure:"stacktrace"`
	//添加调用者信息
	AddCaller bool `json:",default=true" mapstructure:"addCaller"`
	//调用链，往上多少级 ，在一些中间件，对日志有包装，可以通过这个选项指定。
	CallerShip int `json:",default=3" mapstructure:"callerShip"`
	//输出到哪里标准输出console,还是文件file
	Mode string `json:",default=console" mapstructure:"mode"`
	//文件名称加路径
	FileName string `json:",optional" mapstructure:"filename"`
	//error级别的日志输入到不同的地方,默认console 输出到标准错误输出，file可以指定文件
	ErrorFileName string `json:",optional" mapstructure:"errorFileName"`
	// 日志文件大小 单位MB 默认500MB
	MaxSize int `json:",optional" mapstructure:"maxSize"`
	//日志保留天数
	MaxAge int `json:",optional" mapstructure:"maxAge"`
	//日志最大保留的个数
	MaxBackup int `json:",optional" mapstructure:"maxBackUp"`
	//异步日志 日志将先输入到内存到，定时批量落盘。如果设置这个值，要保证在程序退出的时候调用Sync(),在开发阶段不用设置为true。
	Async bool `json:",optional" mapstructure:"async"`
	//是否输出json格式
	Json bool `json:",optional" mapstructure:"json"`
	//是否日志压缩
	Compress bool `json:",optional" mapstructure:"compress"`
	// file 模式是否输出到控制台
	Console bool `json:"console" mapstructure:"console"`
	// 非json格式，是否加上颜色。
	Color bool `json:",default=true" mapstructure:"color"`
	//是否report
	IsReport bool `json:",optional" mapstructure:"isReport"`
	//report配置
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
		//当存储的格式为JSON的时候这些作为可以key
		MessageKey:    "message",
		LevelKey:      "level",
		TimeKey:       "time",
		NameKey:       "logger",
		CallerKey:     "caller",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		//以上字段输出的格式
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
	//是否加上颜色。
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
	//文件模式同时输出到控制台
	if lc.Mode == _file && lc.Console {
		consoleWs := zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), zapcore.ErrorLevel)
		c = append(c, consoleWs)
	}
	if lc.IsReport {
		//上报的格式一律json
		if !lc.Json {
			encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		}
		//指定级别的日志上报。
		highCore := zapcore.NewCore(encoder, report.NewReportWriterBuffer(lc.ReportConfig), lc.ReportConfig.Level)
		c = append(c, highCore)
	}

	core := zapcore.NewTee(c...)

	logger := zap.New(core)
	//是否新增调用者信息
	if lc.AddCaller {
		lc.options = append(lc.options, zap.AddCaller())
		if lc.CallerShip != 0 {
			lc.options = append(lc.options, zap.AddCallerSkip(lc.CallerShip))
		}
	}
	//当错误时是否添加堆栈信息
	if lc.Stacktrace {
		//在error级别以上添加堆栈
		lc.options = append(lc.options, zap.AddStacktrace(zap.ErrorLevel))
	}
	if lc.Name != "" {
		logger = logger.With(zap.String("project", lc.Name))
	}

	return logger.WithOptions(lc.options...)

}

func CustomTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02-15:04:05"))
}

// StringToLogLevelHookFunc viper的string转zapcore.Level
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

```

```go
func TestViperConfig(t *testing.T) {
    v := viper.New()
    v.SetConfigFile("./config.yaml")
    if err := v.ReadInConfig(); err != nil {
       log.Panicf("read config file failed, err:%v\n", err)
    }
    var c Config
    if err := v.Unmarshal(&c, viper.DecodeHook(StringToLogLevelHookFunc())); err != nil {
       log.Panicf("Unmarshal config file failed, err:%v\n", err)
    }
    InitZapLogger(&c)
    Debug("debug level ", zap.Any("test", "test"))
    Info("info level ", zap.Any("test", "test"))
    Warn("warn level ", zap.Any("test", "test"))
    Error("error level ", zap.Any("test", "test"))
    Panic("panic level ", zap.Any("test", "test"))
    Sync()
}
```

![](https://cdn.learnku.com/uploads/images/202403/13/51993/SeMBezQqkX.png!large)

![](https://cdn.learnku.com/uploads/images/202403/13/51993/Jp1jwtz2qT.png!large)