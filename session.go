package toogo

import (
	"fmt"
	"github.com/toophy/doors/help"
)

// 单独日志消息
type msgListen struct {
	msg  string // 消息
	name string // 别名
	id   uint   // 网络会话ID
	info string // 描述信息
}

// 消息节点(list节点)
type Msg_node struct {
	Len   uint32 // 包长度
	Token uint32 // 包令牌
	Count uint32 // 包内消息数
	Data  []byte // 数据
}

// 发送消息给唯一go程
// 从网络接口接收数据
// 发送数据给合适的go线程->Actor模式(邮箱)
// 邮箱在哪里? ReadSilk决定还是网络端口决定?
// 由ReadSilk决定更能解耦网络端口
type Session struct {
	Id         uint             // 会话ID
	Name       string           // 别名
	mailId     uint             // 邮箱ID
	toMailId   uint             // 目标邮箱ID, 接收到消息都转发到这个邮箱
	typeName   string           // 类型
	ipAddress  string           // 网址(或远程网址)
	connClient *net.TCPConn     // 网络连接
	connListen *net.TCPListener // 侦听连接
}

func (this *Session) InitListen(tid uint, address string, conn *net.TCPListener) {
	this.typeName = "listen"
	this.mailId = GetThreadMsgs().AllocId()
	this.toMailId = tid
	this.ipAddress = address
	this.connClient = conn
}

func (this *Session) InitConn(tid uint, address string, conn *net.TCPListener) {
	this.typeName = "conn"
	this.mailId = GetThreadMsgs().AllocId()
	this.toMailId = tid
	this.ipAddress = address
	this.connClient = conn
}

func (this *Session) Run() {
	go this.RunReader()
	go this.runWriter()
}

func (this *Session) runReader() {
	// 捕捉异常
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case error:
				println("Session::runReader:" + r.(error).Error())
			case string:
				println("Session::runReader:" + r.(string))
			}
		}

		GetApp().CloseSession(this.toMailId, this)
	}()

	for {
		data, ret := this.readConnData(c.Conn)

		if ret == nil {
			// 校验 data.Token, 拆包, 解密, 分别处理消息
			// 解密后, data大小不会有多大变化(只会变小)
			PostThreadMsg(this.toMailId, &data)
		} else {
			PostThreadMsg(tid, msgListen{"read failed", c.Name, c.Id, ret.Error()})
			break
		}
	}

	GetApp().CloseSession(this.toMailId, this)
}

// 读取网络消息
func (this *Session) readConnData(conn *net.TCPConn) (msg Msg_node, ret error) {

	const ConnHeaderSize = 4
	var header [ConnHeaderSize]byte
	var length int
	length, ret = io.ReadFull(conn, header[:])

	if length != ConnHeaderSize {
		fmt.Printf("Net packet header : %d != %d\n", length, ConnHeaderSize)
		return
	}
	if ret != nil {
		return
	}

	msg.Len = (uint32(header[0])) | (uint32(header[1]) << 8)
	msg.Token = uint32(header[2])
	msg.Count = uint32(header[3])

	fmt.Printf("ReadConnData : len =%d, token=%d, count=%d\n", msg.Len, msg.Token, msg.Count)

	// 根据 msg.Len 分配一个 缓冲, 并读取 body
	buf := make([]byte, msg.Len)
	length, ret = io.ReadFull(conn, buf[:])
	if length != ConnHeaderSize {
		fmt.Printf("Net packet body : %d != %d\n", length, msg.Len)
		return
	}
	if ret != nil {
		return
	}

	msg.Data = buf

	return
}

func (this *Session) runWriter() {
	// 捕捉异常
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case error:
				println("Session::runWriter:" + r.(error).Error())
			case string:
				println("Session::runWriter:" + r.(string))
			}
		}

		GetApp().CloseSession(this.toMailId, this)
	}()

	for {
		header := DListNode{}
		header.Init(nil)

		GetThreadMsgs().WaitMsg(this.MailId, &header)

		for {
			n := header.Next
			if n.IsEmpty() {
				break
			}

			t := n.Data.(*Msg_node)

			if t.Len > MaxHeader && t.Len < MaxSendDataLen {
				_, err := this.connClient.Write(t.Data[:MaxHeader+t.Len])
				if err != nil {
					println(err.Error())
				}
			}

			n.Pop()
		}
	}

	GetApp().CloseSession(this.toMailId, this)
}

func (this *Session) PostOneMsg(d interface{}) {
	n := &DListNode{}
	n.Init(d)
	GetThreadMsgs().PushOneMsg(this.MailId, n)
}

func (this *Session) PostMsgList(d *DListNode) {
	GetThreadMsgs().PushMsg(this.MailId, d)
}
