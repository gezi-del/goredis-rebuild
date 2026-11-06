package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// 1.在log文件中，我们主要要实现四种不同严重程度的日志
// Errorf 表示 Error：程序发生错误
// Warnf 表示 Warning：警告但能运行
// Infof 表示 Info：运行信息
// Debugf 表示 Debug：调试信息
// 这四个功能室日志文件主要要实现的功能，在编程组件要先思考这个文件的功能是什么，先把接口写好，方便捋清思路和后续维护

type Logger interface {
	Errorf(format string, v ...interface{}) // interface{}表示任意类型，... 表示可变参数
	Warnf(format string, v ...interface{})
	Infof(format string, v ...interface{})
	Debugf(format string, v ...interface{})
}

// -----------------------------------------------------------------
// 配置部分Option
// 2.接下来我们要思考：一个日志系统需要的配置有哪些，大致考虑以下几点：
// Options:
// |-- 日志等级
// |-- 日志保存在哪里
// |-- 单个日志文件最大多大
// |-- 保存多久
// |-- 保存多少个旧文件
// |__ 是否要压缩日志
// 日志系统不仅仅要打印日志，还需要记录日志文件并滚动管理
type Options struct {
	LogName    string // 日志名称
	LogLevel   string // 级别
	FileName   string // 文件名
	MaxAge     int    // 保留时间，单位为天
	MaxSize    int    // 保留大小，单位为 M
	MaxBackups int    // 保留文件个数
	Compress   bool   // 是否压缩
}

// Option 选项方法，用于按需修改其中的某些配置，但要记得有下面的默认配置
type Option func(*Options)

// 3.既然有配置，就要有初始化的默认配置，类似构造函数
// 思路：先生成默认配置，再用传进来的 Option 覆盖默认值，最后返回配置
func NewOptions(opts ...Option) Options { // 接收参数为任意数量的 Option
	// 这里是初始化，即默认配置
	options := Options{
		LogName:    "app",
		LogLevel:   "info",
		FileName:   "app.log",
		MaxAge:     10,
		MaxSize:    100,
		MaxBackups: 3,
		Compress:   true,
	}

	for _, opt := range opts { // 注意range是有index的，所以要有_
		opt(&options) // opt本身是个函数，然后要修改上面的默认配置，传入一个地址才能修改
	}

	return options
}

// 日志级别的Option函数
// 思路：接收一个用户指定的值然后修改 Options
func WithLogLevel(level string) Option {
	return func(o *Options) {
		o.LogLevel = level
	}
}

// 日志文件修改
// 注意这里用了闭包，内部的匿名函数使用了外部变量，因此返回出去的匿名函数仍旧能记住filename
func WithFileName(filename string) Option {
	return func(o *Options) {
		o.FileName = filename
	}
}

// ------------------------------------------
// zap 是 Uber 开源的一个日志库，主要用于高性能、分级、结构化日志记录
// zap 部分，是真正负责写日志的部分，刚刚的option是用来调整配置的
// 4. 搭建zap
// zap
// │
// ├── Logger
// ├── SugaredLogger
// │
// └── zapcore
//
//	├── Level
//	├── Core
//	├── Encoder
//	├── WriteSyncer
//	└── ...
//
// 建立 Option 到 zap 的 level 映射转换
var Levels = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel, // zapcore 是 zap 的底层核心组件
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
	"fatal": zapcore.FatalLevel,
}

// 定义一个Wrapper，我们使用一个结构体包装zap中的一个日志包装器，变成我们自己的 Logger
type zapLoggerWrapper struct {
	*zap.SugaredLogger         // 这是zap库中的一个日志包装器，负责打印日志
	options            Options // 这里的知识点：embedding，即结构体中套结构体；保存日志配置
}

// 5.Encoder 来决定日志的输出格式, Wrapper 的 method
func (w *zapLoggerWrapper) getEncoder() zapcore.Encoder { // Encoder 就是zapcore内置的组件，决定一条日志被格式化成什么样子
	encoderConfig := zap.NewProductionEncoderConfig()     // 创建zap默认的日志编码配置
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // 修改时间输出格式为容易阅读的ISO 8601 风格

	// 在日志文件中使用大写字母记录日志级别 info -> INFO
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// NewConsoleEncoder 可以打印更符合人们观察的方式, Console 风格
	return zapcore.NewConsoleEncoder(encoderConfig) // 传参是刚刚创建好的配置
	// e.g. 2026-09-08T16:00:00+0800    INFO    redis server started
	//      2026-09-08T16:00:02+0800    DEBUG   received command SET
	//      2026-09-08T16:00:03+0800    ERROR   database error
	// 还有一种常见的 Json 风格：zapcore.NewJSONEncoder(encoderConfig)
	// e.g. {"level":"INFO","time":"2026-09-08T16:00:00+0800","msg":"redis server started"}
}

// 函数告知zap日志写在哪                        可以写日志的目标
func (w *zapLoggerWrapper) getLogWriter() zapcore.WriteSyncer {
	return zapcore.AddSync(&lumberjack.Logger{ // lumberjack是一个实现日志滚动/切割的库
		// 全都是配置Options中的内容
		Filename:   w.options.FileName,
		MaxAge:     w.options.MaxAge,
		MaxSize:    w.options.MaxSize,
		MaxBackups: w.options.MaxBackups,
		Compress:   w.options.Compress,
	})
}

// 6.接下来我们把前面的组件组合起来
// 现在我们有如下组件已经准备好：
// - Options: 日志的配置
// - getEncoder(): 日志格式
// - getLogWriter(): 日志写到哪里
// - Levels: 字符串日志级别 -> zap 日志级别
// - zapLoggerWrapper: 项目自己的日志包装器
// 那么接下来的目标就是把这些组件全部组装起来，让他能跑一个完整流程：
func NewLogger(options Options) Logger {
	w := &zapLoggerWrapper{options: options} // 创建 zapLoggerWrapper
	encoder := w.getEncoder()                // 创建日志编码器来决定日志长什么样子
	writeSyncer := w.getLogWriter()          // 日志写入器：决定日志写入哪里
	// 创建核心组件,也就是把上边两个组件和level层级（允许那些级别输出）串起来
	//             zapcore.Core
	//              │
	//   ┌──────────┼───────────┐
	//   ↓          ↓           ↓
	//Encoder   WriteSyncer    Level
	//   │          │           │
	//格式化      写文件      日志过滤
	core := zapcore.NewCore( // NewCore就是组成核心组件的意思
		encoder,
		writeSyncer,
		Levels[options.LogLevel],
	)

	// 创建实际的zap Logger
	// 前面提到过，SugaredLogger是真正进行日志输出的一个函数，所以我们刚刚搭建的core也只是把参数准备好了
	w.SugaredLogger = zap.New(
		core,                 // 核心组件
		zap.AddCaller(),      // 一个在日志中记录是哪一个文件的哪一行代码导致打印的日志，可用于排查错误
		zap.AddCallerSkip(1), // 不加这个会定位到包装层
	).Sugar() // zap.New()返回的是*zap.Logger,跟我们定义的Logger接口有出入，所以我们转成跟我们一样的sugaredLogger模式

	return w
}

// 7.写全局默认 Logger
var (
	defaultLogger Logger
)

// Go 程序加载这个package log时init()会自动执行
func init() {
	defaultLogger = NewLogger(NewOptions())
}

// import log package
//        ↓
//     init()
//        ↓
// NewOptions()
//        ↓
// NewLogger()
//        ↓
// defaultLogger

// 8. 获取默认Logger,其作用时文件对外暴露默认日志实例的入口
func GetDefaultLogger() Logger {
	return defaultLogger
}
