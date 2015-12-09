package toogo

const (
	packetHeaderSize    = 4 // 小消息包头大小,Len + Count + key : 4字节
	packetBigHeaderSize = 5 // 大消息包头大小,Len + Count : 5字节
	msgHeaderSize       = 4 // 消息头, Len + MsgId : 4字节
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
	Count      uint16 // 包内消息数
	MsgID      uint16 // 当前消息ID
}

// 初始化包
func (this *PacketWriter) InitWriter(d []byte) {
	this.Init(d)
	this.LastMsgPos = packetHeaderSize
	this.Count = 0
	this.MsgID = 0
	this.Pos = packetHeaderSize
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
func (this *PacketWriter) PacketWriteOver(pckType uint16) {
	packet_len := this.Pos
	token := uint32(0)

	old_pos := this.Pos
	this.Pos = 0

	switch pckType {
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
