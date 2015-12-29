package toogo

import (
	"fmt"
	"strconv"
)

// 线程日志 : 生成日志头
func (this *Thread) MakeLogHeader() {
	id_str := strconv.Itoa(int(this.Get_thread_id()))
	this.log_Header[logDebugLevel] = this.log_TimeString + " [D] " + id_str + " "
	this.log_Header[logInfoLevel] = this.log_TimeString + " [I] " + id_str + " "
	this.log_Header[logWarnLevel] = this.log_TimeString + " [W] " + id_str + " "
	this.log_Header[logErrorLevel] = this.log_TimeString + " [E] " + id_str + " "
	this.log_Header[logFatalLevel] = this.log_TimeString + " [F] " + id_str + " "
}

// 线程日志 : 调试[D]级别日志
func (this *Thread) LogDebug(f string, v ...interface{}) {
	this.logBase(logDebugLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 信息[I]级别日志
func (this *Thread) LogInfo(f string, v ...interface{}) {
	this.logBase(logInfoLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 警告[W]级别日志
func (this *Thread) LogWarn(f string, v ...interface{}) {
	this.logBase(logWarnLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 错误[E]级别日志
func (this *Thread) LogError(f string, v ...interface{}) {
	this.logBase(logErrorLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 致命[F]级别日志
func (this *Thread) LogFatal(f string, v ...interface{}) {
	this.logBase(logFatalLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 手动分级日志
func (this *Thread) logBase(level int, info string) {
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

		if this.is_master_thread() {
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
func (this *Thread) add_log(d string) {
	if this.is_master_thread() {
		this.log_FileBuff.WriteString(d)
	}
}
