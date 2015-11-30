package toogo

import (
	"net"
	"os"
	"sync"
	"time"
)

// 这个函数, 后期优化, 需要使用 automic 进行等待写入, 主要是强占目标线程会比较厉害, 但是写入操作非常快
func PostThreadMsg(tid uint32, a IThreadMsg) {
	n := new(DListNode)
	n.Init(a)

	if !a.AddNode(n) {
		println("PostThreadMsg AddNode failed")
		return
	}

	GetThreadMsgs().PushOneMsg(tid, n)
	println("PostThreadMsg")
}

// 创建App
func newApp() *App {
	a := new(App)
	a.sessions = make(map[uint32]*Session, 20)
	a.sessionNames = make(map[string]*Session, 16)
	a.config.ListenPorts = make(map[string]listenPort, 3)
	a.config.ConnectPorts = make(map[string]connectPort, 9)
	return a
}

// 需求
// 1. 主线程一个, 享有 1 号线程
// 2. 管理网络会话
// 3. 整合框架
type App struct {
	master         IThread             // 主线程
	lastSessionId  uint32              // 网络会话ID
	sessions       map[uint32]*Session // 网络会话池
	sessionNames   map[string]*Session // 网络会话池(别名)
	sessionMutex   sync.RWMutex        // 网络会话池读写锁
	config         toogoConfig         // 配置信息
	wg             sync.WaitGroup      // App退出信号组
	gThreadMsgPool *ThreadMsgPool      // 线程间消息池
}

// 应用初始化
func Run(m IThread) {
	ToogoApp.master = m
	if ToogoApp.master != nil {
		ToogoApp.master.Run_thread()
	} else {
		println("Can not find master thread")
		os.Exit(2)
	}

	cfg := &ToogoApp.config

	ToogoApp.gThreadMsgPool = new(ThreadMsgPool)
	ToogoApp.gThreadMsgPool.Init(cfg.MsgPoolCount)

	// Listen
	for _, v := range cfg.ListenPorts {
		Listen(Tid_world, v.Name, v.NetType, v.Address, v.AcceptQuit)
	}

	// Connect
	for _, v := range cfg.ConnectPorts {
		Connect(Tid_world, v.Name, v.NetType, v.Address)
	}

	ToogoApp.wg.Wait()
	<-time.After(6 * time.Second)
	println("quit " + ToogoApp.config.AppName)
}

// 进入线程
func EnterThread() {
	ToogoApp.wg.Add(1)
}

// 离开线程
func LeaveThread() {
	ToogoApp.wg.Done()
}

// 通过Id获取会话对象
func GetConnById(id uint32) *Session {
	ToogoApp.sessionMutex.RLock()
	defer ToogoApp.sessionMutex.RUnlock()

	if v, ok := ToogoApp.sessions[id]; ok {
		return v
	}

	return nil
}

// 通过别名获取会话对象
func GetConnByName(name string) *Session {
	ToogoApp.sessionMutex.RLock()
	defer ToogoApp.sessionMutex.RUnlock()

	if v, ok := ToogoApp.sessionNames[name]; ok {
		return v
	}

	return nil
}

// 新建一个网络会话, 可以使用别名
func newSession(name string) *Session {
	s := new(Session)

	ToogoApp.sessionMutex.Lock()
	defer ToogoApp.sessionMutex.Unlock()

	if len(name) > 0 {
		if _, ok := ToogoApp.sessionNames[name]; ok {
			return nil
		} else {
			s.Name = name
			ToogoApp.sessionNames[name] = s
		}
	}

	s.Id = ToogoApp.lastSessionId
	ToogoApp.sessions[ToogoApp.lastSessionId] = s
	ToogoApp.lastSessionId++

	return s
}

// 删除一个网络会话
func delSession(id uint32) {
	ToogoApp.sessionMutex.Lock()
	defer ToogoApp.sessionMutex.Unlock()

	if _, ok := ToogoApp.sessions[id]; ok {
		delete(ToogoApp.sessions, id)
	}
}

// 建立一个侦听服务
// tid        : 关联线程
// name       : 会话别名
// net_type   : 会话类型(tcp,udp)
// address    : 远程服务ip地址
// accpetQuit : 接收器失败, 就退出
func Listen(tid uint32, name, net_type, address string, accpetQuit bool) {
	println("listen start")
	EnterThread()
	go func(tid uint32, name, net_type, address string, accpetQuit bool) {
		defer LeaveThread()

		// 捕捉异常
		defer func() {
			if r := recover(); r != nil {
				switch r.(type) {
				case error:
					println("Listen:" + r.(error).Error())
				case string:
					println("Listen:" + r.(string))
				}
			}
			// 需要把 panic 信息 写入文件中
		}()

		if len(address) == 0 || len(address) == 0 || len(net_type) == 0 {
			println("listen failed")
			PostThreadMsg(tid, &msgListen{msg: "listen failed", name: name, id: 0, info: "listen failed"})
			return
		}

		// 打开本地TCP侦听
		serverAddr, err := net.ResolveTCPAddr(net_type, address)

		if err != nil {
			println("Listen Start : port failed: '" + address + "' " + err.Error())
			PostThreadMsg(tid, &msgListen{msg: "listen failed", name: name, id: 0, info: "Listen Start : port failed: '" + address + "' " + err.Error()})
			return
		}

		listener, err := net.ListenTCP(net_type, serverAddr)
		if err != nil {
			println("TcpSerer ListenTCP: " + err.Error())
			PostThreadMsg(tid, &msgListen{msg: "listen failed", name: name, id: 0, info: "TcpSerer ListenTCP: " + err.Error()})
			return
		}

		ln := newSession(name)
		ln.InitListen(tid, address, listener)

		println("listen ok")
		PostThreadMsg(tid, &msgListen{msg: "listen ok", name: name, id: 0, info: ""})

		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				if accpetQuit {
					println("accpectQuit")
					break
				}
				continue
			}
			c := newSession("")
			c.InitConn(tid, "", conn)
			c.Run()
			println("accept ok")
			PostThreadMsg(tid, &msgListen{msg: "accept ok", name: "", id: c.Id, info: ""})
		}
		println("listen end")
	}(tid, name, net_type, address, accpetQuit)
}

// 连接一个远程服务
// tid      : 关联线程
// name     : 会话别名
// net_type : 会话类型(tcp,udp)
// address  : 远程服务ip地址
func Connect(tid uint32, name, net_type, address string) {
	EnterThread()
	go func(tid uint32, name, net_type, address string) {
		defer LeaveThread()

		// 捕捉异常
		defer func() {
			if r := recover(); r != nil {
				switch r.(type) {
				case error:
					println("Connect:" + r.(error).Error())
				case string:
					println("Connect:" + r.(string))
				}
			}
			// 需要把 panic 信息 写入文件中
		}()

		if len(address) == 0 || len(net_type) == 0 || len(name) == 0 {
			PostThreadMsg(tid, &msgListen{msg: "connect failed", name: name, id: 0, info: "listen failed"})
			return
		}

		// 打开本地TCP侦听
		remoteAddr, err := net.ResolveTCPAddr(net_type, address)

		if err != nil {
			PostThreadMsg(tid, &msgListen{msg: "connect failed", name: name, id: 0, info: "Connect Start : port failed: '" + address + "' " + err.Error()})
			return
		}

		conn, err := net.DialTCP(net_type, nil, remoteAddr)
		if err != nil {
			PostThreadMsg(tid, &msgListen{msg: "connect failed", name: name, id: 0, info: "Connect dialtcp failed: '" + address + "' " + err.Error()})
		} else {
			c := newSession(name)
			c.InitConn(tid, "", conn)
			c.Run()
			PostThreadMsg(tid, &msgListen{msg: "connect ok", name: name, id: c.Id, info: ""})
		}
	}(tid, name, net_type, address)
}

// 关闭一个会话
// tid      : 关联线程
// s        : 会话对象
func CloseSession(tid uint32, s *Session) {
	EnterThread()
	go func(tid uint32, s *Session) {
		defer LeaveThread()

		// 捕捉异常
		defer func() {
			if r := recover(); r != nil {
				switch r.(type) {
				case error:
					println("CloseSession:" + r.(error).Error())
				case string:
					println("CloseSession:" + r.(string))
				}
			}
			// 需要把 panic 信息 写入文件中
		}()

		PostThreadMsg(tid, &msgListen{msg: "pre close", name: s.Name, id: s.Id, info: ""})

		var err error

		switch s.typeName {
		case "listen":
			err = s.connListen.Close()
		case "conn":
			err = s.connClient.Close()
		}

		if err != nil {
			PostThreadMsg(tid, &msgListen{msg: "close failed", name: s.Name, id: s.Id, info: err.Error()})
		} else {
			PostThreadMsg(tid, &msgListen{msg: "close ok", name: s.Name, id: s.Id, info: ""})
		}

		delSession(s.Id)
	}(tid, s)
}

func GetThreadMsgs() *ThreadMsgPool {
	return ToogoApp.gThreadMsgPool
}
