package toogo

import (
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
	Id         uint32           // 会话ID
	MailId     uint32           // 邮箱ID
	toMailId   uint32           // 目标邮箱ID, 接收到消息都转发到这个邮箱
	PacketType uint16           // 数据包类型:CG,SS,SG
	ConnType   uint16           // 类型
	Name       string           // 别名
	ipAddress  string           // 网址(或远程网址)
	connClient *net.TCPConn     // 网络连接
	connListen *net.TCPListener // 侦听连接
}

func (this *Session) initListen(typ uint32, tid uint32, address string, conn *net.TCPListener) {
	this.Type = typ
	this.ConnType = SessionConn_Listen
	this.MailId, _ = GetThreadMsgs().AllocId()
	this.toMailId = tid
	this.ipAddress = address
	this.connListen = conn
}

func (this *Session) initConn(typ uint32, tid uint32, address string, conn *net.TCPConn) {
	this.Type = typ
	this.ConnType = SessionConn_Connect
	this.MailId, _ = GetThreadMsgs().AllocId()
	this.toMailId = tid
	this.ipAddress = address
	this.connClient = conn
}

func (this *Session) run() {
	if this.Type == SessionPacket_CG {
		EnterThread()
		go this.runReader()

		EnterThread()
		go this.runWriter()
	} else {
		EnterThread()
		go this.runBigReader()

		EnterThread()
		go this.runBigWriter()
	}
}

func (this *Session) runReader() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runReader:")

	for {
		data, err := this.readConnData(this.connClient)

		if err == nil {
			// 校验 data.Token, 拆包, 解密, 分别处理消息
			// 解密后, data大小不会有多大变化(只会变小)
			PostThreadMsg(this.toMailId, &data)
		} else {
			PostThreadMsg(this.toMailId, &Tmsg_net{"read failed", this.Name, this.Id, err.Error()})
			break
		}
	}

	CloseSession(this.toMailId, this.Id)
}

// 读取网络消息
func (this *Session) readConnData(conn *net.TCPConn) (msg Tmsg_packet, err error) {

	var header [packetHeaderSize]byte
	var length int
	length, err = io.ReadFull(conn, header[:])

	if length != packetHeaderSize {
		LogWarnPost(this.MailId, "Net packet header : %d != %d\n", length, packetHeaderSize)
		return
	}
	if err != nil {
		return
	}

	msg.SessionId = this.Id
	msg.PacketType = this.PacketType

	msg.Len = (uint32(header[0])) | (uint32(header[1]) << 8)
	msg.Token = uint32(header[2])
	msg.Count = uint32(header[3])

	// 根据 msg.Len 分配一个 缓冲, 并读取 body
	body_len := msg.Len - packetHeaderSize
	buf := make([]byte, body_len)
	length, err = io.ReadFull(conn, buf[:])
	if length != int(body_len) {
		LogWarnPost(this.MailId, "Net packet body : %d != %d\n", length, body_len)
		return
	}
	if err != nil {
		LogWarnPost(this.MailId, "Seesion read body : readlen=%d , body=%d\n", length, body_len)
		return
	}

	msg.Data = buf

	return
}

func (this *Session) runWriter() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runWriter:")

	for {
		header := DListNode{}
		header.Init(nil)

		GetThreadMsgs().WaitMsg(this.MailId, &header)
		for {

			n := header.Next
			if n.IsEmpty() {
				break
			}

			t := n.Data.(*Tmsg_packet)

			if t.Len > packetHeaderSize {
				_, err := this.connClient.Write(t.Data[:t.Len])
				if err != nil {
					LogWarnPost(this.MailId, err.Error())
				}
			}

			n.Pop()
		}
	}

	CloseSession(this.toMailId, this.Id)
}

func (this *Session) runBigReader() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runBigReader:")

	for {
		data, err := this.readBigConnData(this.connClient)

		if err == nil {
			// 校验 data.Token, 拆包, 解密, 分别处理消息
			// 解密后, data大小不会有多大变化(只会变小)
			PostThreadMsg(this.toMailId, &data)
		} else {
			PostThreadMsg(this.toMailId, &Tmsg_net{"read failed", this.Name, this.Id, err.Error()})
			break
		}
	}

	CloseSession(this.toMailId, this.Id)
}

// 读取网络消息
func (this *Session) readBigConnData(conn *net.TCPConn) (msg Tmsg_packet, err error) {

	var header [packetBigHeaderSize]byte
	var length int
	length, err = io.ReadFull(conn, header[:])

	if length != packetBigHeaderSize {
		LogWarnPost(this.MailId, "Net packet header : %d != %d\n", length, packetBigHeaderSize)
		return
	}
	if err != nil {
		return
	}

	msg.SessionId = this.Id
	msg.PacketType = this.PacketType

	msg.Len = uint32(header[0]) | (uint32(header[1]) << 8) | (uint32(header[2]) << 16)
	msg.Count = (uint32(header[3])) | (uint32(header[4]) << 8)

	// 根据 msg.Len 分配一个 缓冲, 并读取 body
	body_len := msg.Len - packetBigHeaderSize
	buf := make([]byte, body_len)
	length, err = io.ReadFull(conn, buf[:])
	if length != int(body_len) {
		LogWarnPost(this.MailId, "Net packet body : %d != %d\n", length, body_len)
		return
	}
	if err != nil {
		LogWarnPost(this.MailId, "Seesion read body : readlen=%d , body=%d\n", length, body_len)
		return
	}

	msg.Data = buf

	return
}

func (this *Session) runBigWriter() {
	defer LeaveThread()
	defer RecoverCommon(this.MailId, "Session::runBigWriter:")

	for {
		header := DListNode{}
		header.Init(nil)

		GetThreadMsgs().WaitMsg(this.MailId, &header)
		for {

			n := header.Next
			if n.IsEmpty() {
				break
			}

			t := n.Data.(*Tmsg_packet)

			if t.Len > packetBigHeaderSize {
				_, err := this.connClient.Write(t.Data[:t.Len])
				if err != nil {
					LogWarnPost(this.MailId, err.Error())
				}
			}

			n.Pop()
		}
	}

	CloseSession(this.toMailId, this.Id)
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
func Listen(typ uint32, tid uint32, name, net_type, address string, accpetQuit bool) {
	EnterThread()
	go func(tid uint32, name, net_type, address string, accpetQuit bool) {
		defer LeaveThread()
		defer RecoverCommon(0, "Listen:")

		if len(address) == 0 || len(address) == 0 || len(net_type) == 0 {
			LogWarnPost(0, "listen failed")
			PostThreadMsg(tid, &Tmsg_net{"listen failed", name, 0, "listen failed"})
			return
		}

		// 打开本地TCP侦听
		serverAddr, err := net.ResolveTCPAddr(net_type, address)

		if err != nil {
			LogWarnPost(0, "Listen Start : port failed: '"+address+"' "+err.Error())
			PostThreadMsg(tid, &Tmsg_net{"listen failed", name, 0, "Listen Start : port failed: '" + address + "' " + err.Error()})
			return
		}

		listener, err := net.ListenTCP(net_type, serverAddr)
		if err != nil {
			LogWarnPost(0, "TcpSerer ListenTCP: "+err.Error())
			PostThreadMsg(tid, &Tmsg_net{"listen failed", name, 0, "TcpSerer ListenTCP: " + err.Error()})
			return
		}

		ln := newSession(name)
		ln.initListen(typ, tid, address, listener)

		LogInfoPost(0, "listen ok")
		PostThreadMsg(tid, &Tmsg_net{"listen ok", name, 0, ""})

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
			PostThreadMsg(tid, &Tmsg_net{"accept ok", "", c.Id, ""})
		}
		LogInfoPost(0, "listen end")
	}(tid, name, net_type, address, accpetQuit)
}

// 连接一个远程服务
// tid      : 关联线程
// name     : 会话别名
// net_type : 会话类型(tcp,udp)
// address  : 远程服务ip地址
func Connect(typ uint32, tid uint32, name, net_type, address string) {
	EnterThread()
	go func(tid uint32, name, net_type, address string) {
		defer LeaveThread()
		defer RecoverCommon(0, "Connect:")

		if len(address) == 0 || len(net_type) == 0 || len(name) == 0 {
			PostThreadMsg(tid, &Tmsg_net{"connect failed", name, 0, "listen failed"})
			return
		}

		// 打开本地TCP侦听
		remoteAddr, err := net.ResolveTCPAddr(net_type, address)

		if err != nil {
			PostThreadMsg(tid, &Tmsg_net{"connect failed", name, 0, "Connect Start : port failed: '" + address + "' " + err.Error()})
			return
		}

		conn, err := net.DialTCP(net_type, nil, remoteAddr)
		if err != nil {
			PostThreadMsg(tid, &Tmsg_net{"connect failed", name, 0, "Connect dialtcp failed: '" + address + "' " + err.Error()})
		} else {
			c := newSession(name)
			c.initConn(typ, tid, "", conn)
			c.run()
			PostThreadMsg(tid, &Tmsg_net{"connect ok", name, c.Id, ""})
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

			PostThreadMsg(tid, &Tmsg_net{"pre close", s.Name, s.Id, ""})

			var err error

			if s.connListen != nil {
				err = s.connListen.Close()
			} else if s.connClient != nil {
				err = s.connClient.Close()
			}

			if err != nil {
				PostThreadMsg(tid, &Tmsg_net{"close failed", s.Name, s.Id, err.Error()})
			} else {
				PostThreadMsg(tid, &Tmsg_net{"close ok", s.Name, s.Id, ""})
			}

			delSession(s.Id)
		}

	}(tid, sessionId)
}
