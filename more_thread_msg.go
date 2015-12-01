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
type msgListen struct {
	msg  string // 消息
	name string // 别名
	id   uint32 // 网络会话ID
	info string // 描述信息
}

func (this *msgListen) Exec(home interface{}) bool {
	home.(IThread).LogDebug("msg=%s,name=%s,session=%d,info=%s", this.msg, this.name, this.id, this.info)
	// LogInfoPost(0, "msg=%s,name=%s,session=%d,info=%s", this.msg, this.name, this.id, this.info)
	return true
}

// 消息节点(list节点)
type Msg_node struct {
	Len   uint32 // 包长度
	Token uint32 // 包令牌
	Count uint32 // 包内消息数
	Data  []byte // 数据
}

func (this *Msg_node) Exec(home interface{}) bool {
	LogInfoPost(0, "Msg_node")
	return true
}
