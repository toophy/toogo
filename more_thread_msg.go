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
	home.(IThread).AddLog(this.Data)
	return true
}

// 单独日志消息
type Tmsg_net struct {
	SessionId uint64 // 网络会话ID
	Msg       string // 消息
	Name      string // 别名
	Info      string // 描述信息
}

func (this *Tmsg_net) Exec(home interface{}) bool {
	return home.(IThread).On_netEvent(this)
}

// 消息节点(list节点)
type Tmsg_packet struct {
	Tgid       uint64 // 中转标记
	SessionId  uint64 // 会话ID
	Len        uint32 // 包长度
	Token      uint32 // 包令牌
	Count      uint16 // 包内消息数
	PacketType uint16 // 会话类型
	Data       []byte // 数据
}

func (this *Tmsg_packet) Exec(home interface{}) bool {
	switch this.PacketType {
	case SessionPacket_C2G:
		return home.(IThread).procC2GNetPacket(this)
	case SessionPacket_G2C:
		return home.(IThread).procC2GNetPacket(this)
	case SessionPacket_S2G:
		return home.(IThread).procS2GNetPacket(this)
	case SessionPacket_G2S:
		return home.(IThread).procG2SNetPacket(this)
	}

	return false
}
