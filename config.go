package toogo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// toogo服务器引擎App
var ToogoApp *App

// toogo服务器引擎配置
type toogoConfig struct {
	AppName         string                 // App名称
	workPath        string                 // 工作目录
	MsgPoolCount    uint32                 // ThreadMsgPool邮箱容量
	LogFlushTime    int64                  // 日志写入缓冲的间隔时间(毫秒)
	LogLimitLevel   int                    // 显示这个等级之上的日志(控制台)
	LogBuffMax      int                    // 日志缓冲(KB)
	LogFileBuffSize int                    // 日志文件缓冲(KB)
	LogDir          string                 // 日志目录
	LogFileName     string                 // 日志文件名
	ProfFileName    string                 // 性能分析文件名
	ListenPorts     map[string]listenPort  // 众多侦听端口
	ConnectPorts    map[string]connectPort // 众多远程连接端口
}

// 侦听端口
type listenPort struct {
	Name       string // 别称
	Address    string // 侦探所在IP地址和端口
	NetType    string // tcp | udp
	PacketType uint16 // CG | SS | SG
	AcceptQuit bool   // Accept失败退出
}

// 远程连接端口
type connectPort struct {
	Name       string // 别称
	Address    string // 侦探所在IP地址和端口
	NetType    string // tcp | udp
	PacketType uint16 // CG | SS | SG
}

func init() {
	ToogoApp = newApp()

	var iniConfig IniConfig
	cc, err := iniConfig.parseFile("conf/toogo.conf")
	if err != nil {
		cc, err = iniConfig.parseFile("bin/conf/toogo.conf")
		if err != nil {
			fmt.Println("Can not find conf/toogo.conf")
			os.Exit(1)
		} else {
			ToogoApp.rootPath = "bin/"
		}
	} else {
		ToogoApp.rootPath = ""
	}

	cfg := &ToogoApp.config

	cfg.AppName = cc.DefaultString("sys::AppName", "toogo")

	workPath, _ := os.Getwd()
	workPath, _ = filepath.Abs(workPath)
	cfg.workPath = workPath

	cfg.MsgPoolCount = uint32(cc.DefaultInt("sys::ThreadMsgPoolCount", 10000))

	cfg.LogFlushTime = cc.DefaultInt64("log::LogFlushTime", 300)

	cfg.LogLimitLevel = cc.DefaultInt("log::LogLimitLevel", logDebugLevel)

	cfg.LogBuffMax = cc.DefaultInt("log::LogBuffMax", 20*1024) * 1024

	cfg.LogFileBuffSize = cc.DefaultInt("log::LogFileBuffSize", 10*1024) * 1024

	cfg.LogDir = cc.DefaultString("log::LogDir", "../log")

	cfg.LogFileName = cfg.LogDir + "/" +
		cc.DefaultString("log::LogFileName", cfg.AppName+".log")

	cfg.ProfFileName = cfg.LogDir + "/" +
		cc.DefaultString("test::ProfFileName", cfg.AppName+".prof.log")

	// listen address
	for i := 1; i <= 9; i++ {
		sec := fmt.Sprintf("listen_%d::", i)
		name := cc.DefaultString(sec+"Name", "")
		if len(name) > 0 {
			address := cc.DefaultString(sec+"Address", "")
			if len(address) > 0 {
				lp := listenPort{Name: name, Address: address}
				lp.NetType = cc.DefaultString(sec+"NetType", "tcp")
				lp.AcceptQuit = cc.DefaultBool(sec+"AcceptQuit", true)
				pt := cc.DefaultString(sec+"PacketType", "CG")
				switch strings.ToUpper(pt) {
				case "CG":
					lp.PacketType = SessionPacket_C2G
				case "GC":
					lp.PacketType = SessionPacket_G2C
				case "GS":
					lp.PacketType = SessionPacket_G2S
				case "SG":
					lp.PacketType = SessionPacket_S2G
				}
				cfg.ListenPorts[name] = lp
			}
		}
	}

	// connect address
	for i := 1; i <= 9; i++ {
		sec := fmt.Sprintf("connect_%d::", i)
		name := cc.DefaultString(sec+"Name", "")
		if len(name) > 0 {
			address := cc.DefaultString(sec+"Address", "")
			if len(address) > 0 {
				lp := connectPort{Name: name, Address: address}
				lp.NetType = cc.DefaultString(sec+"NetType", "tcp")
				pt := cc.DefaultString(sec+"PacketType", "CG")
				switch strings.ToUpper(pt) {
				case "CG":
					lp.PacketType = SessionPacket_C2G
				case "GC":
					lp.PacketType = SessionPacket_G2C
				case "GS":
					lp.PacketType = SessionPacket_G2S
				case "SG":
					lp.PacketType = SessionPacket_S2G
				}
				cfg.ConnectPorts[name] = lp
			}
		}
	}
}
