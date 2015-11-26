package toogo

import (
	"os"
	"path/filepath"
)

const (
	LogBuffSize = 10 * 1024 * 1024
	LogDir      = "../log"
	ProfFile    = "pangu_prof.log"
	LogFileName = LogDir + "/pangu.log"
)

const (
	MaxDataLen     = 5080
	MaxSendDataLen = 4000
	MaxHeader      = 2
)

const (
	LogDebugLevel = 0                // 日志等级 : 调试信息
	LogInfoLevel  = 1                // 日志等级 : 普通信息
	LogWarnLevel  = 2                // 日志等级 : 警告信息
	LogErrorLevel = 3                // 日志等级 : 错误信息
	LogFatalLevel = 4                // 日志等级 : 致命信息
	LogMaxLevel   = 5                // 日志最大等级
	LogLimitLevel = LogInfoLevel     // 显示这个等级之上的日志(控制台)
	LogBuffMax    = 20 * 1024 * 1024 // 日志缓冲
)

const (
	Tid_world    = iota // 世界线程
	Tid_screen_1        // 场景线程1
	Tid_screen_2        // 场景线程2
	Tid_screen_3        // 场景线程3
	Tid_screen_4        // 场景线程4
	Tid_screen_5        // 场景线程5
	Tid_screen_6        // 场景线程6
	Tid_screen_7        // 场景线程7
	Tid_screen_8        // 场景线程8
	Tid_screen_9        // 场景线程9
	Tid_net_1           // 网络线程1
	Tid_net_2           // 网络线程2
	Tid_net_3           // 网络线程3
	Tid_db_1            // 数据库线程1
	Tid_db_2            // 数据库线程2
	Tid_db_3            // 数据库线程3
	Tid_last            // 最终线程ID
)

const (
	Evt_gap_time  = 16     // 心跳时间(毫秒)
	Evt_gap_bit   = 4      // 心跳时间对应得移位(快速运算使用)
	Evt_lay1_time = 160000 // 第一层事件池最大支持时间(毫秒)
)

const (
	UpdateCurrTimeCount = 32 // 刷新时间戳变更上线
)

const ConnHeaderSize = 4

var ToogoApp *App

type toogoConfig struct {
	AppName      string
	workPath     string
	MsgPoolCount uint  // ThreadMsgPool邮箱容量
	LogFlushTime int64 // 日志写入缓冲的间隔时间(毫秒)
}

func init() {
	ToogoApp = newApp()

	var iniConfig IniConfig
	cc, err := iniConfig.parseFile("conf/toogo.conf")
	if err != nil {
		println("Can not find conf/toogo.conf")
		os.Exit(1)
	}

	ToogoApp.config.AppName = cc.DefaultString("AppName", "toogo")

	workPath, _ := os.Getwd()
	workPath, _ = filepath.Abs(workPath)
	ToogoApp.config.workPath = workPath

	ToogoApp.config.MsgPoolCount = uint(cc.DefaultInt("ThreadMsgPoolCount", 10000))

	ToogoApp.config.LogFlushTime = cc.DefaultInt64("LogFlushTime", 300)
}
