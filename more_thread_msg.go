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
	home.(IThread).Add_log(this.Data)
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
	return home.(IThread).On_NetEvent(this)
}

// 消息节点(list节点)
type Tmsg_packet struct {
	Len   uint32 // 包长度
	Token uint32 // 包令牌
	Count uint32 // 包内消息数
	Data  []byte // 数据
}

func (this *Tmsg_packet) Exec(home interface{}) bool {
	return home.(IThread).On_NetPacket(this)
}
