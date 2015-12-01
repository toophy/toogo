package toogo

import (
	"fmt"
	"strconv"
	"time"
)

// 线程日志 : 调试[D]级别日志
func LogDebugPost(src_id uint32, f string, v ...interface{}) {
	logBasePost(src_id, logDebugLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 信息[I]级别日志
func LogInfoPost(src_id uint32, f string, v ...interface{}) {
	logBasePost(src_id, logInfoLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 警告[W]级别日志
func LogWarnPost(src_id uint32, f string, v ...interface{}) {
	logBasePost(src_id, logWarnLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 错误[E]级别日志
func LogErrorPost(src_id uint32, f string, v ...interface{}) {
	logBasePost(src_id, logErrorLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 致命[F]级别日志
func LogFatalPost(src_id uint32, f string, v ...interface{}) {
	logBasePost(src_id, logFatalLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 手动分级日志
func logBasePost(src_id uint32, level int, info string) {

	if level < logDebugLevel || level > logFatalLevel {
		return
	}

	lenInfo := len(info)
	if lenInfo < 1 {
		return
	}

	timeString := time.Now().Format("15:04:05")

	id_str := strconv.Itoa(int(src_id))
	headerString := ""

	switch level {
	case logDebugLevel:
		headerString = timeString + " [D] " + id_str + " "
	case logInfoLevel:
		headerString = timeString + " [I] " + id_str + " "
	case logWarnLevel:
		headerString = timeString + " [W] " + id_str + " "
	case logErrorLevel:
		headerString = timeString + " [E] " + id_str + " "
	case logFatalLevel:
		headerString = timeString + " [F] " + id_str + " "
	}

	s := ""
	if info[lenInfo-1:] == "\n" {
		s = headerString + info
	} else {
		s = headerString + info + "\n"
	}

	//s = strings.Replace(s, "\n", "\n"+headerString, -1) + "\n"

	if level >= ToogoApp.config.LogLimitLevel {
		fmt.Print(s)
	}

	PostThreadMsg(Tid_master, &msgThreadLog{Data: s})
}
