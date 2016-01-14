package toogo

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	logDebugLevel = 0 // 日志等级 : 调试信息
	logInfoLevel  = 1 // 日志等级 : 普通信息
	logWarnLevel  = 2 // 日志等级 : 警告信息
	logErrorLevel = 3 // 日志等级 : 错误信息
	logFatalLevel = 4 // 日志等级 : 致命信息
	logMaxLevel   = 5 // 日志最大等级
)

// 线程日志接口
type ILogOs interface {
	AddLog(d string)                     //增加日志信息
	LogDebug(f string, v ...interface{}) // 线程日志 : 调试[D]级别日志
	LogInfo(f string, v ...interface{})  // 线程日志 : 信息[I]级别日志
	LogWarn(f string, v ...interface{})  // 线程日志 : 警告[W]级别日志
	LogError(f string, v ...interface{}) // 线程日志 : 错误[E]级别日志
	LogFatal(f string, v ...interface{}) // 线程日志 : 致命[F]级别日志
}

// 线程日志系统
type LogOs struct {
	threadId       uint32              // 线程Id号
	threadIdStr    string              // 线程Id号字符串
	log_Buffer     []byte              // 线程日志缓冲
	log_BufferLen  int                 // 线程日志缓冲长度
	log_TimeString string              // 时间格式(精确到秒2015.08.13 16:33:00)
	log_Header     [logMaxLevel]string // 各级别日志头
	log_FileBuff   bytes.Buffer        // 日志总缓冲, Tid_master才会使用
	log_FileHandle *os.File            // 日志文件, Tid_master才会使用
	log_FlushTime  int64               // 日志文件最后写入时间
}

func (this *LogOs) InitLog(threadId uint32) error {
	this.threadId = threadId
	this.threadIdStr = strconv.Itoa(int(this.threadId))
	// 日志初始化
	this.log_Buffer = make([]byte, ToogoApp.config.LogBuffMax)
	this.log_BufferLen = 0

	this.UpdateLogTimeHeader()

	if this.threadId == Tid_master {
		this.log_FileBuff.Grow(ToogoApp.config.LogFileBuffSize)

		if !IsExist(ToogoApp.config.LogFileName) {
			os.Create(ToogoApp.config.LogFileName)
		}
		file, err := os.OpenFile(ToogoApp.config.LogFileName, os.O_RDWR, os.ModePerm)
		if err != nil {
			return err
		}
		this.log_FileHandle = file
		this.log_FileHandle.Seek(0, 2)
		// 第一条日志
		this.LogDebug("          {%s}启动成功", ToogoApp.config.AppName)
	}

	return nil
}

// 线程日志 : 刷新日志数据到文件
func (this *LogOs) FlushToFile(curr_time int64) {
	if this.threadId == Tid_master && this.log_FlushTime < curr_time {
		this.log_FlushTime = curr_time + ToogoApp.config.LogFlushTime
		if this.log_FileBuff.Len() > 0 {
			this.log_FileHandle.Write(this.log_FileBuff.Bytes())
			this.log_FileBuff.Reset()
		}
	}
}

// 线程日志 : 更新时间标记头
func (this *LogOs) UpdateLogTimeHeader() {
	this.log_TimeString = time.Now().Format("15:04:05")

	this.log_Header[logDebugLevel] = this.log_TimeString + " [D] " + this.threadIdStr + " "
	this.log_Header[logInfoLevel] = this.log_TimeString + " [I] " + this.threadIdStr + " "
	this.log_Header[logWarnLevel] = this.log_TimeString + " [W] " + this.threadIdStr + " "
	this.log_Header[logErrorLevel] = this.log_TimeString + " [E] " + this.threadIdStr + " "
	this.log_Header[logFatalLevel] = this.log_TimeString + " [F] " + this.threadIdStr + " "
}

// 线程日志 : 发送线程间消息
func (this *LogOs) SendThreadLog() {
	// 发送日志到日志线程
	if this.threadId != Tid_master && this.log_BufferLen > 0 {
		PostThreadMsg(Tid_master, &msgThreadLog{Data: string(this.log_Buffer[:this.log_BufferLen])})
		copy(this.log_Buffer[:0], "")
		this.log_BufferLen = 0
	}
}

// 线程日志 : 调试[D]级别日志
func (this *LogOs) LogDebug(f string, v ...interface{}) {
	this.logBase(logDebugLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 信息[I]级别日志
func (this *LogOs) LogInfo(f string, v ...interface{}) {
	this.logBase(logInfoLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 警告[W]级别日志
func (this *LogOs) LogWarn(f string, v ...interface{}) {
	this.logBase(logWarnLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 错误[E]级别日志
func (this *LogOs) LogError(f string, v ...interface{}) {
	this.logBase(logErrorLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 致命[F]级别日志
func (this *LogOs) LogFatal(f string, v ...interface{}) {
	this.logBase(logFatalLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 手动分级日志
func (this *LogOs) logBase(level int, info string) {
	lenInfo := len(info)
	if lenInfo < 1 {
		return
	}

	if level >= logDebugLevel && level < logMaxLevel {
		s := ""
		if info[lenInfo-1:] == "\n" {
			s = this.log_Header[level] + info
		} else {
			s = this.log_Header[level] + info + "\n"
		}

		if this.threadId == Tid_master {
			this.log_FileBuff.WriteString(s)
		} else {
			s_len := len(s)
			copy(this.log_Buffer[this.log_BufferLen:], s)
			this.log_BufferLen += s_len
		}

		if level >= ToogoApp.config.LogLimitLevel {
			fmt.Print(s)
		}
	} else {
		fmt.Println("logBase : level failed : ", level)
	}
}

// 增加日志到缓冲
func (this *LogOs) AddLog(d string) {
	if this.threadId == Tid_master {
		this.log_FileBuff.WriteString(d)
	}
}
