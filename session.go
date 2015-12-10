package toogo

import (
	// "errors"
	"fmt"
	"io"
	"net"
)

// 封包处理(大批量,或者一个网络口(代理))
// 简化发送消息的代码
// Lua支持消息包
// Lua支持线程间消息
//
// Session上面做标记, 大包, 小包 等
// 每种包会有不同的解包机制, 包头大小
// 如何在accecpt时就知道这个包属于哪种包头?
// 是一个什么消息么?
// 比如, 一开始默认都是小包
// 验证成功后, 根据对方资质, 变成大包
// 或者
// 有一个Listen侦听到的都是服务器连接, 都是大包头?
// 其实只有gate服连接才是大包头, 其他都是小包头
//
// 服务器之间都是大包头, 也即是包长度4字节, 包消息数量2字节,
// 只有gate和客户端之间才是小包头, 包长度2字节, 包消息数量1字节,
//
// 只有服务器和gate服的连接才会包中有包, 其他都是简单的消息处理
//
// session需要分清楚
// 1. 这是什么连接(服务器,客户端)
// 2. 服务器的连接中, 是否有gate服
//    a. CG连接 小包
//    b. SS连接 大包
//    c. GS连接 混合大包
//

const (
	SessionConn_Listen  = 0 // 侦听
	SessionConn_Connect = 1 // 主动连接
)

// 发送消息给唯一go程
// 从网络接口接收数据
// 发送数据给合适的go线程->Actor模式(邮箱)
// 邮箱在哪里? ReadSilk决定还是网络端口决定?
// 由ReadSilk决定更能解耦网络端口
type Session struct {
	SessionId  uint64           // 会话ID
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
	EnterThread()
	go this.runReader()
	EnterThread()
	go this.runWriter()
}

func (this *Session) runReader() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runReader:")

	headerSize := uint32(pckCGHeaderSize)
	switch this.PacketType {
	case SessionPacket_CG:
		headerSize = uint32(pckCGHeaderSize)
	case SessionPacket_SS:
		headerSize = uint32(pckSSHeaderSize)
	case SessionPacket_SG:
		headerSize = uint32(pckSGHeaderSize)
	}

	var err error
	header := make([]byte, headerSize)
	var length int
	var xStream Stream
	xStream.Init(header)

	for {
		length, err = io.ReadFull(this.connClient, header[:])

		if length != int(headerSize) || err != nil {
			if err == nil {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, fmt.Sprintf("Net packet header : %d != %d\n", length, headerSize)})
			} else {
				PostThreadMsg(this.toMailId, &Tmsg_net{this.SessionId, "read failed", this.Name, err.Error()})
			}
			break
		}

		msg := new(Tmsg_packet)
		msg.SessionId = this.SessionId
		msg.PacketType = this.PacketType

		xStream.Seek(0)
		switch this.PacketType {
		case SessionPacket_CG:
			msg.Len = uint32(xStream.ReadUint16())
			msg.Token = uint32(xStream.ReadUint8())
			msg.Count = uint16(xStream.ReadUint8())
		case SessionPacket_SS:
			msg.Len = uint32(xStream.ReadUint24())
			msg.Count = uint16(xStream.ReadUint16())
		case SessionPacket_SG:
			msg.Len = uint32(xStream.ReadUint24())
			msg.Count = uint16(xStream.ReadUint16())
		}

		// 根据 msg.Len 分配一个 缓冲, 并读取 body
		body_len := msg.Len - headerSize
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

func (this *Session) runWriter() {

	headerSize := uint32(pckCGHeaderSize)
	switch this.PacketType {
	case SessionPacket_CG:
		headerSize = uint32(pckCGHeaderSize)
	case SessionPacket_SS:
		headerSize = uint32(pckSSHeaderSize)
	case SessionPacket_SG:
		headerSize = uint32(pckSGHeaderSize)
	}

	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runWriter:")

	for {
		header := DListNode{}
		header.Init(nil)

		GetThreadMsgs().WaitMsg(this.MailId, &header)
		if header.IsEmpty() {
			break
		}

		for {

			n := header.Next
			if n.IsEmpty() {
				break
			}

			t := n.Data.(*Tmsg_packet)

			if t.Len > headerSize {
				start_pos := 0
				for i := 0; i < 6; i++ {
					wLen, err := this.connClient.Write(t.Data[start_pos:t.Len])
					if err != nil {
						LogWarnPost(this.MailId, err.Error())
						CloseSession(this.toMailId, this.SessionId)
						return
					}
					if uint32(wLen) == t.Len {
						break
					}
					start_pos = wLen
				}
			}

			n.Pop()
		}
	}

	CloseSession(this.toMailId, this.SessionId)
}

// 通过Id获取会话对象
func GetSessionById(id uint64) *Session {
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
func delSession(id uint64) {
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
func CloseSession(tid uint32, sessionId uint64) {
	EnterThread()
	go func(tid uint32, sessionId uint64) {
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

// 创建一个长度的PacketWriter
func NewPacket(l int, sessionId uint64) *PacketWriter {
	defer RecoverCommon(0, "toogo::NewPacket:")

	session := GetSessionById(sessionId)
	if session != nil {
		p := new(PacketWriter)
		d := make([]byte, l)
		p.InitWriter(d, session.PacketType, session.MailId)

		return p
	}

	return nil
}

// 发送网络消息包
func SendPacket(p *PacketWriter) bool {

	defer RecoverCommon(0, "toogo::SendPacket:")

	p.PacketWriteOver()
	x := new(Tmsg_packet)
	x.Data = p.GetData()
	x.Len = uint32(p.GetPos())
	x.Count = uint16(p.Count)

	PostThreadMsg(p.MailId, x)

	return false
}
