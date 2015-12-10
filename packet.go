package toogo

const (
	packetHeaderSize = 4 // 小消息包头大小,Len + Count + key : 4字节
	SessionPacket_CG = 0 // 客户端和Gate连接
	SessionPacket_SS = 1 // 服务器和服务器连接
	SessionPacket_SG = 2 // 服务器和Gate连接
	pckCGHeaderSize  = 4 // CG 类型包头长度
	pckSSHeaderSize  = 5 // SS 类型包头长度
	pckSGHeaderSize  = 5 // SG 类型包头长度
	msgHeaderSize    = 4 // 消息头长度
)

// 操作网络封包
type PacketReader struct {
	Stream
	Data  []byte // 数据
	Count uint16 // 包内消息数
}

// 初始化包
func (this *PacketReader) InitReader(d []byte, count uint16) {
	this.Init(d)
	this.Count = count
	this.Pos = 0
}

// 读入消息ID
func (this *PacketReader) ReadMsgId() {
	// this.ReadUint16()
}

// 开始读取一个网络封包
func (this *PacketReader) BeginRead() {
}

// 操作网络封包
type PacketWriter struct {
	Stream
	LastMsgPos uint64 // 最近一个消息终止位置
	Data       []byte // 数据
	MailId     uint32 // 会话邮箱
	Count      uint16 // 包内消息数
	MsgID      uint16 // 当前消息ID
	PacketType uint16 // 会话类型
}

// 初始化包
func (this *PacketWriter) InitWriter(d []byte, pckType uint16, mailId uint32) {
	this.Init(d)
	this.Count = 0
	this.MsgID = 0
	this.PacketType = pckType
	headerSize := pckCGHeaderSize
	switch this.PacketType {
	case SessionPacket_SS:
		headerSize = pckSSHeaderSize
	case SessionPacket_SG:
		headerSize = pckSGHeaderSize
	}
	this.LastMsgPos = uint64(headerSize)
	this.Pos = uint64(headerSize)
	this.MailId = mailId
}

// 写入消息ID
func (this *PacketWriter) WriteMsgId(id uint16) {
	this.MsgID = id
	this.Pos = this.Pos + msgHeaderSize
}

// 写入一个消息
func (this *PacketWriter) WriteMsgOver() {
	// 当前长度
	msg_sum_len := uint32(this.Pos - this.LastMsgPos)

	old_pos := this.Pos
	this.Pos = this.LastMsgPos
	this.WriteUint32((uint32(this.MsgID)<<16 | msg_sum_len))
	this.Pos = old_pos
	this.LastMsgPos = old_pos
	this.Count++
}

// 结束一个封包
func (this *PacketWriter) PacketWriteOver() {
	packet_len := this.Pos
	token := uint32(0)

	old_pos := this.Pos
	this.Pos = 0

	switch this.PacketType {
	case SessionPacket_CG:
		this.WriteUint16(uint16(packet_len))
		this.WriteUint8(uint8(token))
		this.WriteUint8(uint8(this.Count))
	case SessionPacket_SS:
		this.WriteUint24(uint32(packet_len))
		this.WriteUint16(this.Count)
	case SessionPacket_SG:
		this.WriteUint24(uint32(packet_len))
		this.WriteUint16(this.Count)
	}

	this.Pos = old_pos
}
