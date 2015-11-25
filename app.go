package toogo

import (
	"sync"
)

// 这个函数, 后期优化, 需要使用 automic 进行等待写入, 主要是强占目标线程会比较厉害, 但是写入操作非常快
func PostThreadMsg(tid uint, data interface{}) {
	n := new(DListNode)
	n.Init(data)
	GetThreadMsgs().PushOneMsg(tid, n)
}

// 需求
// 1. 主线程一个, 享有 1 号线程
// 2. 管理网络会话
// 3. 整合框架
type App struct {
	name          string              // 应用名称
	master        IThread             // 主线程
	lastSessionId uint                // 网络会话ID
	sessions      map[uint]*Session   // 网络会话池
	sessionNames  map[string]*Session // 网络会话池(别名)
	sessionMutex  sync.RWMutex        // 网络会话池读写锁
}

var g_App *App

// 调用者保证需线程安全
func GetApp() *App {
	if g_App == nil {
		g_App = new(App)
	}
	return g_App
}

// 应用初始化
func (this *App) Init(name string, m IThread) {
	this.name = name
	this.master = m
}

// 通过Id获取会话对象
func (this *App) GetConnById(id uint) *Session {
	this.sessionMutex.RLock()
	defer this.sessionMutex.RUnlock()

	if v, ok := this.sessions[id]; ok {
		return v
	}

	return nil
}

// 通过别名获取会话对象
func (this *App) GetConnByName(name string) *Session {
	this.sessionMutex.RLock()
	defer this.sessionMutex.RUnlock()

	if v, ok := this.sessionNames[name]; ok {
		return v
	}

	return nil
}

// 新建一个网络会话, 可以使用别名
func (this *App) newSession(name string) *Session {
	s := new(Session)

	this.sessionMutex.Lock()
	defer this.sessionMutex.Unlock()

	if len(name) > 0 {
		if ok, _ := this.sessionNames[name]; ok {
			return nil
		} else {
			s.Name = name
			this.sessionNames[name] = s
		}
	}

	s.Id = this.lastSessionId
	this.sessions[this.lastSessionId] = s
	this.lastSessionId++

	return s
}

// 删除一个网络会话
func (this *App) delSession(id int) {
	this.sessionMutex.Lock()
	defer this.sessionMutex.Unlock()

	if _, ok := this.sessions[id]; ok {
		delete(this.sessions, id)
	}
}

// 建立一个侦听服务
// tid        : 关联线程
// name       : 会话别名
// net_type   : 会话类型(tcp,udp)
// address    : 远程服务ip地址
// accpetQuit : 接收器失败, 就退出
func (this *App) Listen(tid uint, name, net_type, address string, accpetQuit bool) {
	go func(tid uint, name, net_type, address string, accpetQuit bool) {
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

		ln := this.newSession(name)
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
			c := this.newSession("")
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
func (this *App) Connect(tid uint, name, net_type, address string) {
	go func(tid uint, name, net_type, address string) {
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
			c := this.newSession(name)
			c.InitConn(tid, "", conn)
			c.Run()
			PostThreadMsg(tid, msgListen{"connect ok", name, c.Id, ""})
		}
	}(tid, name, net_type, address)
}

// 关闭一个会话
// tid      : 关联线程
// s        : 会话对象
func (this *App) CloseSession(tid uint, s *Session) {
	go func(tid uint, s *Session) {
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

		this.delSession(s.Id)
	}(tid, s)
}
