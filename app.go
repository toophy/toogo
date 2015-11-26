package toogo

import (
	"net"
	"sync"
)

// 这个函数, 后期优化, 需要使用 automic 进行等待写入, 主要是强占目标线程会比较厉害, 但是写入操作非常快
func PostThreadMsg(tid uint32, data interface{}) {
	n := new(DListNode)
	n.Init(data)
	GetThreadMsgs().PushOneMsg(tid, n)
}

// 创建App
func newApp() *App {
	a := new(App)
	return a
}

// 需求
// 1. 主线程一个, 享有 1 号线程
// 2. 管理网络会话
// 3. 整合框架
type App struct {
	master        IThread             // 主线程
	lastSessionId uint32              // 网络会话ID
	sessions      map[uint32]*Session // 网络会话池
	sessionNames  map[string]*Session // 网络会话池(别名)
	sessionMutex  sync.RWMutex        // 网络会话池读写锁
	config        toogoConfig         // 配置信息
}

// 应用初始化
func Run(m IThread) {
	ToogoApp.master = m
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
	go func(tid uint32, name, net_type, address string, accpetQuit bool) {
		if len(address) == 0 || len(address) == 0 || len(net_type) == 0 {
			PostThreadMsg(tid, msgListen{"listen failed", name, 0, "listen failed"})
			return
		}

		// 打开本地TCP侦听
		serverAddr, err := net.ResolveTCPAddr(net_type, address)

		if err != nil {
			PostThreadMsg(tid, msgListen{"listen failed", name, 0, "Listen Start : port failed: '" + address + "' " + err.Error()})

			return
		}

		listener, err := net.ListenTCP(net_type, serverAddr)
		if err != nil {
			PostThreadMsg(tid, msgListen{"listen failed", name, 0, "TcpSerer ListenTCP: " + err.Error()})
			return
		}

		ln := newSession(name)
		ln.InitListen(tid, address, listener)

		PostThreadMsg(tid, msgListen{"listen ok", name, 0, ""})

		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				if accpetQuit {
					break
				}
				continue
			}
			c := newSession("")
			c.InitConn(tid, "", conn)
			c.Run()
			PostThreadMsg(tid, msgListen{"accept ok", "", c.Id, ""})
		}
	}(tid, name, net_type, address, accpetQuit)
}

// 连接一个远程服务
// tid      : 关联线程
// name     : 会话别名
// net_type : 会话类型(tcp,udp)
// address  : 远程服务ip地址
func Connect(tid uint32, name, net_type, address string) {
	go func(tid uint32, name, net_type, address string) {
		if len(address) == 0 || len(net_type) == 0 || len(name) == 0 {
			PostThreadMsg(tid, msgListen{"connect failed", name, 0, "listen failed"})
			return
		}

		// 打开本地TCP侦听
		remoteAddr, err := net.ResolveTCPAddr(net_type, address)

		if err != nil {
			PostThreadMsg(tid, msgListen{"connect failed", name, 0, "Connect Start : port failed: '" + address + "' " + err.Error()})
			return
		}

		conn, err := net.DialTCP(net_type, nil, remoteAddr)
		if err != nil {
			PostThreadMsg(tid, msgListen{"connect failed", name, 0, "Connect dialtcp failed: '" + address + "' " + err.Error()})
		} else {
			c := newSession(name)
			c.InitConn(tid, "", conn)
			c.Run()
			PostThreadMsg(tid, msgListen{"connect ok", name, c.Id, ""})
		}
	}(tid, name, net_type, address)
}

// 关闭一个会话
// tid      : 关联线程
// s        : 会话对象
func CloseSession(tid uint32, s *Session) {
	go func(tid uint32, s *Session) {
		PostThreadMsg(tid, msgListen{"pre close", s.Name, s.Id, ""})

		var err error

		switch s.typeName {
		case "listen":
			err = s.connListen.Close()
		case "conn":
			err = s.connClient.Close()
		}

		if err != nil {
			PostThreadMsg(tid, msgListen{"close failed", s.Name, s.Id, err.Error()})
		} else {
			PostThreadMsg(tid, msgListen{"close ok", s.Name, s.Id, ""})
		}

		delSession(s.Id)
	}(tid, s)
}
