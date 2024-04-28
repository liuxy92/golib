package zaplog

import (
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Options struct {
	LogLevel      string //日志级别
	LogFileDir    string //日志路径
	AppName       string //Filename是要写入日志的文件前缀
	ErrorFileName string //Error输出日志文件前缀
	WarnFileName  string //Warn输出日志文件前缀
	InfoFileName  string //Info输出日志文件前缀
	DebugFileName string //Debug输出日志文件前缀
	MaxSize       int    //一个文件多少M大于该数字开始切分文件
	MaxBackups    int    //要保留的最大旧日志文件数
	MaxAge        int    //根据日期保留旧日志文件的最大天数
	CutType       int    //日志分割方式
	Development   bool   //日志模式
	zap.Config
}

type Logger struct {
	*zap.SugaredLogger
	sync.RWMutex
	Opts      *Options `json:"opts"`
	zapConfig zap.Config
	inited    bool
}

var (
	logger                         *Logger
	sp                             = string(filepath.Separator) //路径分隔符'/'
	errWS, warnWS, infoWS, debugWS zapcore.WriteSyncer          //IO输出
	debugConsoleWS                 = zapcore.Lock(os.Stdout)    //控制台调试标准输出
	errorConsoleWS                 = zapcore.Lock(os.Stderr)    //控制台异常标准输出
)

func init() {
	logger = &Logger{
		Opts: &Options{},
	}
}

func InitLogger(cfg ...*Options) {
	logger.Lock()
	defer logger.Unlock()
	if logger.inited {
		logger.Info("[initLogger] zaplog already initialized")
		return
	}

	if len(cfg) > 0 {
		logger.Opts = cfg[0]
	}
	logger.loadCfg()
	logger.init()
	logger.Info("[initLogger] zap plugin initializing completed")
	logger.inited = true
}

// GetLogger return logger
func GetLogger() *Logger {
	return logger
}

func (lg *Logger) init() {
	lg.setSyncers()
	var err error
	myLogger, err := lg.zapConfig.Build(lg.cores())
	if err != nil {
		panic(err)
	}
	lg.SugaredLogger = myLogger.Sugar()
	defer lg.SugaredLogger.Sync()
}

func (lg *Logger) loadCfg() {
	if lg.Opts.Development {
		lg.zapConfig = zap.NewDevelopmentConfig()
		lg.zapConfig.EncoderConfig.EncodeTime = timeEncoder
	} else {
		lg.zapConfig = zap.NewProductionConfig()
		lg.zapConfig.EncoderConfig.EncodeTime = timeUnixNano
	}
	if lg.Opts.OutputPaths == nil || len(lg.Opts.OutputPaths) == 0 {
		lg.zapConfig.OutputPaths = []string{"stdout"}
	}
	if lg.Opts.ErrorOutputPaths == nil || len(lg.Opts.ErrorOutputPaths) == 0 {
		lg.zapConfig.ErrorOutputPaths = []string{"stderr"}
	}

	// 设置日志级别
	switch lg.Opts.LogLevel {
	case "debug":
		lg.zapConfig.Level.SetLevel(zap.DebugLevel)
	case "info":
		lg.zapConfig.Level.SetLevel(zap.InfoLevel)
	case "warn":
		lg.zapConfig.Level.SetLevel(zap.WarnLevel)
	case "error":
		lg.zapConfig.Level.SetLevel(zap.ErrorLevel)
	}

	// 默认输出到程序运行目录的logs子目录
	if lg.Opts.LogFileDir == "" {
		lg.Opts.LogFileDir, _ = filepath.Abs(filepath.Dir(filepath.Join(".")))
		lg.Opts.LogFileDir += sp + "logs" + sp
	}
	if lg.Opts.AppName == "" {
		lg.Opts.AppName = "app"
	}
	if lg.Opts.ErrorFileName == "" {
		lg.Opts.ErrorFileName = "error.log"
	}
	if lg.Opts.WarnFileName == "" {
		lg.Opts.WarnFileName = "warn.log"
	}
	if lg.Opts.InfoFileName == "" {
		lg.Opts.InfoFileName = "info.log"
	}
	if lg.Opts.DebugFileName == "" {
		lg.Opts.DebugFileName = "debug.log"
	}
	if lg.Opts.MaxSize == 0 {
		lg.Opts.MaxSize = 100
	}
	if lg.Opts.MaxBackups == 0 {
		lg.Opts.MaxBackups = 30
	}
	if lg.Opts.MaxAge == 0 {
		lg.Opts.MaxAge = 30
	}
}

func (lg *Logger) setSyncers() {
	f := func(fName string) zapcore.WriteSyncer {
		if lg.Opts.CutType == 0 {
			//lumberjack根据文件大小进行切割文件
			return zapcore.AddSync(&lumberjack.Logger{
				Filename:   lg.Opts.LogFileDir + sp + lg.Opts.AppName + "-" + fName, //日志文件的位置
				MaxSize:    lg.Opts.MaxSize,                                         //在进行切割之前，日志文件的最大大小(以MB为单位)
				MaxBackups: lg.Opts.MaxBackups,                                      //保留旧文件的最大个数
				MaxAge:     lg.Opts.MaxAge,                                          //保留旧文件的最大天数
				Compress:   true,                                                    //是否压缩/归档旧文件
				LocalTime:  true,
			})
		} else {
			//每一小时一个文件
			logf, _ := rotatelogs.New(
				lg.Opts.LogFileDir+sp+lg.Opts.AppName+"-"+fName+".%Y_%m%d_%H",
				rotatelogs.WithLinkName(lg.Opts.LogFileDir+sp+lg.Opts.AppName+"-"+fName),
				rotatelogs.WithMaxAge(time.Duration(lg.Opts.MaxAge)*24*time.Hour),
				rotatelogs.WithRotationTime(time.Minute),
			)
			return zapcore.AddSync(logf)
		}
	}
	errWS = f(lg.Opts.ErrorFileName)
	warnWS = f(lg.Opts.WarnFileName)
	infoWS = f(lg.Opts.InfoFileName)
	debugWS = f(lg.Opts.DebugFileName)
	return
}

func (lg *Logger) cores() zap.Option {
	fileEncoder := zapcore.NewJSONEncoder(lg.zapConfig.EncoderConfig)
	//consoleEncoder := zapcore.NewConsoleEncoder(lg.zapConfig.EncoderConfig)
	encoderConfig := zap.NewDevelopmentConfig().EncoderConfig
	encoderConfig.EncodeTime = timeEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	errPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel && zapcore.ErrorLevel-lg.zapConfig.Level.Level() > -1
	})
	warnPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.WarnLevel && zapcore.WarnLevel-lg.zapConfig.Level.Level() > -1
	})
	infoPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.InfoLevel && zapcore.InfoLevel-lg.zapConfig.Level.Level() > -1
	})
	debugPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.DebugLevel && zapcore.DebugLevel-lg.zapConfig.Level.Level() > -1
	})
	cores := []zapcore.Core{
		zapcore.NewCore(fileEncoder, errWS, errPriority),
		zapcore.NewCore(fileEncoder, warnWS, warnPriority),
		zapcore.NewCore(fileEncoder, infoWS, infoPriority),
		zapcore.NewCore(fileEncoder, debugWS, debugPriority),
	}
	if lg.Opts.Development {
		cores = append(cores, []zapcore.Core{
			zapcore.NewCore(consoleEncoder, errorConsoleWS, errPriority),
			zapcore.NewCore(consoleEncoder, debugConsoleWS, warnPriority),
			zapcore.NewCore(consoleEncoder, debugConsoleWS, infoPriority),
			zapcore.NewCore(consoleEncoder, debugConsoleWS, debugPriority),
		}...)
	}
	return zap.WrapCore(func(c zapcore.Core) zapcore.Core {
		return zapcore.NewTee(cores...)
	})
}

func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05"))
}

func timeUnixNano(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendInt64(t.UnixNano() / 1e6)
}
