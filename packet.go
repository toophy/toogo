package toogo

const (
	SessionPacket_C2G   = 0  // 客户端和Gate连接 C端
	SessionPacket_G2C   = 1  // 客户端和Gate连接 G端
	SessionPacket_S2G   = 2  // 普通服务器和Gate连接 S端
	SessionPacket_G2S   = 3  // 普通服务器和Gate连接 G端
	pckC2GHeaderSize    = 4  // C2G 类型包头长度
	pckG2CHeaderSize    = 4  // G2C 类型包头长度
	pckG2SHeaderSize    = 13 // G2S 类型包头长度
	pckS2GHeaderSize    = 5  // S2G 类型包头长度
	pckS2GSubHeaderSize = 13 // S2G 类型包的子包头长度
	msgHeaderSize       = 4  // 消息头长度
)

// 操作网络封包
type PacketReader struct {
	Stream
	CurrMsgPos uint64 // 当前消息开始位置
	CurrMsgLen uint16 // 当前消息长度
	CurrMsgId  uint16 // 当前消息Id
	Count      uint16 // 包内消息数
}

// 初始化包
func (this *PacketReader) InitReader(d []byte, count uint16) {
	this.Init(d)
	this.CurrMsgId = 0
	this.CurrMsgLen = 0
	this.CurrMsgPos = 0
	this.Count = count
}

func (this *PacketReader) PreReadMsg(msg_id uint16, msg_len uint16, start_pos uint64) {
	this.CurrMsgId = msg_id
	this.CurrMsgLen = msg_len
	this.CurrMsgPos = start_pos
}

func (this *PacketReader) GetReadMsg() (msg_id uint16, msg_len uint16, start_pos uint64) {
	return this.CurrMsgId, this.CurrMsgLen, this.CurrMsgPos
}

// 操作网络封包
type PacketWriter struct {
	Stream
	LastMsgPos      uint64 // 最近一个消息终止位置
	MailId          uint32 // 会话邮箱
	Count           uint16 // 包内消息数
	MsgID           uint16 // 当前消息ID
	WritePacketType uint16 // 会话类型
	Tgid            uint64 // 标记
}

// 初始化包
func (this *PacketWriter) InitWriter(d []byte, pckType uint16, mailId uint32) {
	this.Init(d)
	this.Count = 0
	this.MsgID = 0
	this.WritePacketType = pckType
	headerSize := getHeaderSize(this.WritePacketType)
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

	switch this.WritePacketType {
	case SessionPacket_C2G:
		this.WriteUint16(uint16(packet_len))
		this.WriteUint8(uint8(token))
		this.WriteUint8(uint8(this.Count))
	case SessionPacket_G2C:
		this.WriteUint16(uint16(packet_len))
		this.WriteUint16(uint16(this.Count))
	case SessionPacket_G2S:
		this.WriteUint24(uint32(packet_len))
		this.WriteUint16(this.Count)
		this.WriteUint64(this.Tgid)
	case SessionPacket_S2G:
		this.WriteUint24(uint32(packet_len))
		this.WriteUint16(this.Count)
	}

	this.Pos = old_pos
}

// 拷贝一个完整消息
func (this *PacketWriter) CopyMsg(d []byte, dLen uint64) bool {
	defer RecoverCommon(0, "PacketWriter::CopyMsg")

	this.WriteDataEx(d, dLen)
	this.LastMsgPos = this.LastMsgPos + dLen
	this.Count++

	return true
}

// 拷贝定长消息
func (this *PacketWriter) CopyFromPacketReader(r *PacketReader, pos uint64, dLen uint64) bool {
	defer RecoverCommon(0, "PacketWriter::CopyFromPacketReader")

	this.WriteDataEx(r.Data[pos:pos+dLen], dLen)
	this.LastMsgPos = this.LastMsgPos + dLen

	return true
}
