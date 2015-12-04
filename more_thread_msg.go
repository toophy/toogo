package toogo

//
// 消息
//-------------------------------------------------------
type IThreadMsg interface {
	Exec(home interface{}) bool // 执行事件
}

// 单独日志消息
type msgThreadLog struct {
	Data string
}

func (this *msgThreadLog) Exec(home interface{}) bool {
	home.(IThread).add_log(this.Data)
	return true
}

// 单独日志消息
type Tmsg_net struct {
	Msg  string // 消息
	Name string // 别名
	Id   uint32 // 网络会话ID
	Info string // 描述信息
}

func (this *Tmsg_net) Exec(home interface{}) bool {
	return home.(IThread).On_netEvent(this)
}

// 消息节点(list节点)
type Tmsg_packet struct {
	SessionId  uint32 // 会话ID
	Len        uint32 // 包长度
	Token      uint32 // 包令牌
	Count      uint16 // 包内消息数
	PacketType uint16 // 会话类型
	Data       []byte // 数据
}

func (this *Tmsg_packet) Exec(home interface{}) bool {
	if this.PacketType == SessionPacket_CG {
		return home.(IThread).procCGNetPacket(this)
	} else if this.PacketType == SessionPacket_SS {
		return home.(IThread).procSSNetPacket(this)
	} else if this.PacketType == SessionPacket_SG {
		return home.(IThread).procSGNetPacket(this)
	}

	return false
}
