package toogo

import (
	// "errors"
	"fmt"
	"io"
	"net"
)

const (
	SessionPacket_CG    = 0 // 客户端和Gate连接
	SessionPacket_SS    = 1 // 服务器和服务器连接
	SessionPacket_SG    = 2 // 服务器和Gate连接
	SessionConn_Listen  = 0 // 侦听
	SessionConn_Connect = 1 // 主动连接
)

// 发送消息给唯一go程
// 从网络接口接收数据
// 发送数据给合适的go线程->Actor模式(邮箱)
// 邮箱在哪里? ReadSilk决定还是网络端口决定?
// 由ReadSilk决定更能解耦网络端口
type Session struct {
	SessionId  uint32           // 会话ID
	MailId     uint32           // 邮箱ID
	toMailId   uint32           // 目标邮箱ID, 接收到消息都转发到这个邮箱
	PacketType uint16           // 数据包类型:CG,SS,SG
	ConnType   uint16           // 类型
	Name       string           // 别名
	ipAddress  string           // 网址(或远程网址)
	connClient *net.TCPConn     // 网络连接
	connListen *net.TCPListener // 侦听连接
}

func (this *Session) initListen(typ uint16, tid uint32, address string, conn *net.TCPListener) {
	this.PacketType = typ
	this.ConnType = SessionConn_Listen
	this.MailId, _ = GetThreadMsgs().AllocId()
	this.toMailId = tid
	this.ipAddress = address
	this.connListen = conn
}

func (this *Session) initConn(typ uint16, tid uint32, address string, conn *net.TCPConn) {
	this.PacketType = typ
	this.ConnType = SessionConn_Connect
	this.MailId, _ = GetThreadMsgs().AllocId()
	this.toMailId = tid
	this.ipAddress = address
	this.connClient = conn
}

func (this *Session) GetIPAddress() string {
	return this.ipAddress
}

func (this *Session) run() {
	switch this.PacketType {
	case SessionPacket_CG:
		EnterThread()
		go this.runCGReader()
		EnterThread()
		go this.runCGWriter()

	case SessionPacket_SS:
		EnterThread()
		go this.runSSReader()
		EnterThread()
		go this.runSSWriter()

	case SessionPacket_SG:
		EnterThread()
		go this.runSGReader()
		EnterThread()
		go this.runSGWriter()
	}
}

const (
	pckCGHeaderSize = 4  // CG 类型包头长度
	pckSSHeaderSize = 4  // SS 类型包头长度
	pckSGHeaderSize = 13 // SG 类型包头长度
)

func (this *Session) runCGReader() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runCGReader:")

	var err error
	header := make([]byte, pckCGHeaderSize)
	var length int
	var xStream Stream
	xStream.Init(header)

	for {
		length, err = io.ReadFull(this.connClient, header[:])

		if length != pckCGHeaderSize || err != nil {
			if err == nil {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, fmt.Sprintf("Net packet header : %d != %d\n", length, pckCGHeaderSize)})
			} else {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, err.Error()})
			}
			break
		}

		msg := new(Tmsg_cg_packet)
		msg.SessionId = this.SessionId
		msg.PacketType = this.PacketType

		xStream.Seek(0)
		msg.Len = uint32(xStream.ReadUint16())
		msg.Token = uint32(xStream.ReadUint8())
		msg.Count = uint16(xStream.ReadUint8())

		// 根据 msg.Len 分配一个 缓冲, 并读取 body
		body_len := msg.Len - pckCGHeaderSize
		buf := make([]byte, body_len)
		length, err = io.ReadFull(this.connClient, buf[:])
		if length != int(body_len) || err != nil {
			if err == nil {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, fmt.Sprintf("Net packet body : %d != %d\n", length, body_len)})
			} else {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, err.Error()})
			}
			break
		}

		msg.Data = buf

		PostThreadMsg(this.toMailId, msg)
	}

	CloseSession(this.toMailId, this.SessionId)
}

func (this *Session) runCGWriter() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runCGWriter:")

	for {
		header := DListNode{}
		header.Init(nil)

		GetThreadMsgs().WaitMsg(this.MailId, &header)
		for {

			n := header.Next
			if n.IsEmpty() {
				break
			}

			t := n.Data.(*Tmsg_cg_packet)

			if t.Len > pckCGHeaderSize {
				start_pos := 0
				for i := 0; i < 20; i++ {
					wLen, err := this.connClient.Write(t.Data[start_pos:t.Len])
					if err != nil {
						LogWarnPost(this.MailId, err.Error())
						break
					}
					if uint32(wLen) < t.Len {
						start_pos = wLen
					}
				}
			}

			n.Pop()
		}
	}

	CloseSession(this.toMailId, this.SessionId)
}

func (this *Session) runSSReader() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runSSReader:")

	var err error
	header := make([]byte, pckSSHeaderSize)
	var length int
	var xStream Stream
	xStream.Init(header)

	for {
		length, err = io.ReadFull(this.connClient, header[:])

		if length != pckSSHeaderSize || err != nil {
			if err == nil {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, fmt.Sprintf("Net packet header : %d != %d\n", length, pckSSHeaderSize)})
			} else {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, err.Error()})
			}
			break
		}

		msg := new(Tmsg_ss_packet)
		msg.SessionId = this.SessionId
		msg.PacketType = this.PacketType

		xStream.Seek(0)
		msg.Len = uint32(xStream.ReadUint16())
		msg.Token = uint32(xStream.ReadUint8())
		msg.Count = uint16(xStream.ReadUint8())

		// 根据 msg.Len 分配一个 缓冲, 并读取 body
		body_len := msg.Len - pckSSHeaderSize
		buf := make([]byte, body_len)
		length, err = io.ReadFull(this.connClient, buf[:])
		if length != int(body_len) || err != nil {
			if err == nil {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, fmt.Sprintf("Net packet body : %d != %d\n", length, body_len)})
			} else {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, err.Error()})
			}
			break
		}

		msg.Data = buf

		PostThreadMsg(this.toMailId, msg)
	}

	CloseSession(this.toMailId, this.SessionId)
}

func (this *Session) runSSWriter() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runSSWriter:")

	for {
		header := DListNode{}
		header.Init(nil)

		GetThreadMsgs().WaitMsg(this.MailId, &header)
		for {

			n := header.Next
			if n.IsEmpty() {
				break
			}

			t := n.Data.(*Tmsg_ss_packet)

			if t.Len > pckSSHeaderSize {
				start_pos := 0
				for i := 0; i < 20; i++ {
					wLen, err := this.connClient.Write(t.Data[start_pos:t.Len])
					if err != nil {
						LogWarnPost(this.MailId, err.Error())
						break
					}
					if uint32(wLen) < t.Len {
						start_pos = wLen
					}
				}
			}

			n.Pop()
		}
	}

	CloseSession(this.toMailId, this.SessionId)
}

func (this *Session) runSGReader() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runSGReader:")

	var err error
	header := make([]byte, pckSGHeaderSize)
	var length int
	var xStream Stream
	xStream.Init(header)

	for {
		length, err = io.ReadFull(this.connClient, header[:])

		if length != pckSGHeaderSize || err != nil {
			if err == nil {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, fmt.Sprintf("Net packet header : %d != %d\n", length, pckSGHeaderSize)})
			} else {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, err.Error()})
			}
			break
		}

		msg := new(Tmsg_sg_packet)
		msg.SessionId = this.SessionId
		msg.PacketType = this.PacketType

		xStream.Seek(0)
		msg.Len = uint32(xStream.ReadUint24())
		msg.Count = uint16(xStream.ReadUint16())
		msg.Flag = xStream.ReadUint64()

		// 根据 msg.Len 分配一个 缓冲, 并读取 body
		body_len := msg.Len - pckSGHeaderSize
		buf := make([]byte, body_len)
		length, err = io.ReadFull(this.connClient, buf[:])
		if length != int(body_len) || err != nil {
			if err == nil {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, fmt.Sprintf("Net packet body : %d != %d\n", length, body_len)})
			} else {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, err.Error()})
			}
			break
		}

		msg.Data = buf

		PostThreadMsg(this.toMailId, msg)
	}

	CloseSession(this.toMailId, this.SessionId)
}

func (this *Session) runSGWriter() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runSGWriter:")

	for {
		header := DListNode{}
		header.Init(nil)

		GetThreadMsgs().WaitMsg(this.MailId, &header)
		for {

			n := header.Next
			if n.IsEmpty() {
				break
			}

			t := n.Data.(*Tmsg_sg_packet)

			if t.Len > pckSGHeaderSize {
				start_pos := 0
				for i := 0; i < 20; i++ {
					wLen, err := this.connClient.Write(t.Data[start_pos:t.Len])
					if err != nil {
						LogWarnPost(this.MailId, err.Error())
						break
					}
					if uint32(wLen) < t.Len {
						start_pos = wLen
					}
				}
			}

			n.Pop()
		}
	}

	CloseSession(this.toMailId, this.SessionId)
}

// 通过Id获取会话对象
func GetSessionById(id uint32) *Session {
	ToogoApp.sessionMutex.RLock()
	defer ToogoApp.sessionMutex.RUnlock()

	if v, ok := ToogoApp.sessions[id]; ok {
		return v
	}

	return nil
}

// 通过别名获取会话对象
func GetSessionByName(name string) *Session {
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

	s.SessionId = ToogoApp.lastSessionId
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
func Listen(typ uint16, tid uint32, name, net_type, address string, accpetQuit bool) {
	EnterThread()
	go func(tid uint32, name, net_type, address string, accpetQuit bool) {
		defer LeaveThread()
		defer RecoverCommon(0, "Listen:")

		if len(address) == 0 || len(address) == 0 || len(net_type) == 0 {
			LogWarnPost(0, "listen failed")
			PostThreadMsg(tid, &Tmsg_net{0, "listen failed", name, "listen failed"})
			return
		}

		// 打开本地TCP侦听
		serverAddr, err := net.ResolveTCPAddr(net_type, address)

		if err != nil {
			LogWarnPost(0, "Listen Start : port failed: '"+address+"' "+err.Error())
			PostThreadMsg(tid, &Tmsg_net{0, "listen failed", name, "Listen Start : port failed: '" + address + "' " + err.Error()})
			return
		}

		listener, err := net.ListenTCP(net_type, serverAddr)
		if err != nil {
			LogWarnPost(0, "TcpSerer ListenTCP: "+err.Error())
			PostThreadMsg(tid, &Tmsg_net{0, "listen failed", name, "TcpSerer ListenTCP: " + err.Error()})
			return
		}

		ln := newSession(name)
		ln.initListen(typ, tid, address, listener)

		LogInfoPost(0, "listen ok")
		PostThreadMsg(tid, &Tmsg_net{0, "listen ok", name, ""})

		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				if accpetQuit {
					LogInfoPost(0, "accpectQuit")
					break
				}
				continue
			}
			c := newSession("")
			c.initConn(typ, tid, "", conn)
			c.run()
			LogInfoPost(0, "accept ok")
			PostThreadMsg(tid, &Tmsg_net{c.SessionId, "accept ok", "", ""})
		}
		LogInfoPost(0, "listen end")
	}(tid, name, net_type, address, accpetQuit)
}

// 连接一个远程服务
// tid      : 关联线程
// name     : 会话别名
// net_type : 会话类型(tcp,udp)
// address  : 远程服务ip地址
func Connect(typ uint16, tid uint32, name, net_type, address string) {
	EnterThread()
	go func(tid uint32, name, net_type, address string) {
		defer LeaveThread()
		defer RecoverCommon(0, "Connect:")

		if len(address) == 0 || len(net_type) == 0 || len(name) == 0 {
			PostThreadMsg(tid, &Tmsg_net{0, "connect failed", name, "listen failed"})
			return
		}

		// 打开本地TCP侦听
		remoteAddr, err := net.ResolveTCPAddr(net_type, address)

		if err != nil {
			PostThreadMsg(tid, &Tmsg_net{0, "connect failed", name, "Connect Start : port failed: '" + address + "' " + err.Error()})
			return
		}

		conn, err := net.DialTCP(net_type, nil, remoteAddr)
		if err != nil {
			PostThreadMsg(tid, &Tmsg_net{0, "connect failed", name, "Connect dialtcp failed: '" + address + "' " + err.Error()})
		} else {
			c := newSession(name)
			c.initConn(typ, tid, "", conn)
			c.run()
			PostThreadMsg(tid, &Tmsg_net{c.SessionId, "connect ok", name, ""})
		}
	}(tid, name, net_type, address)
}

// 关闭一个会话
// tid      : 关联线程
// s        : 会话对象
func CloseSession(tid uint32, sessionId uint32) {
	EnterThread()
	go func(tid uint32, sessionId uint32) {
		defer LeaveThread()

		s := GetSessionById(sessionId)
		if s != nil {
			defer RecoverCommon(s.MailId, "CloseSession:")

			PostThreadMsg(tid, &Tmsg_net{s.SessionId, "pre close", s.Name, ""})

			var err error

			if s.connListen != nil {
				err = s.connListen.Close()
			} else if s.connClient != nil {
				err = s.connClient.Close()
			}

			if err != nil {
				PostThreadMsg(tid, &Tmsg_net{s.SessionId, "close failed", s.Name, err.Error()})
			} else {
				PostThreadMsg(tid, &Tmsg_net{s.SessionId, "close ok", s.Name, ""})
			}

			delSession(s.SessionId)
		}

	}(tid, sessionId)
}
