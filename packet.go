package toogo

const (
	SessionPacket_C2G   = 0     // 客户端和Gate连接 C端
	SessionPacket_G2C   = 1     // 客户端和Gate连接 G端
	SessionPacket_S2G   = 2     // 普通服务器和Gate连接 S端
	SessionPacket_G2S   = 3     // 普通服务器和Gate连接 G端
	pckC2GHeaderSize    = 4     // C2G 类型包头长度
	pckG2CHeaderSize    = 4     // G2C 类型包头长度
	pckG2SHeaderSize    = 13    // G2S 类型包头长度
	pckS2GHeaderSize    = 5     // S2G 类型包头长度
	pckS2GSubHeaderSize = 13    // S2G 类型包的子包头长度
	msgHeaderSize       = 4     // 消息头长度
	subMsgHeaderSize    = 3     // 子消息头长度
	pckTgidSize         = 8     // tgid标记长度
	pckMsgExtern        = 16000 // 消息Id扩展
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
		this.WriteUint16(this.Count)
	case SessionPacket_G2S:
		this.WriteUint24(uint32(packet_len))
		this.WriteUint16(this.Count)
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

// // 大包
// type PacketBig struct {
// 	targetWriter map[uint64]map[uint64]uint64 // 目标大包
// 	flagWriter   map[uint64]*PacketWriter     // 标记者小包
// }

// 目标
// 1. 建立PacketWriter集合
//    a. 第一层: 投递目标
//    b. 第二层: 投递目标的PacketWriter集合
//    c. 写入一定长度(toogo.conf设置参数)后, 自动启用一个新PacketWriter
//    d.
// 2. 建立线程间消息传送功能集合
// 确定这两个功能从线程中分拆, 可以独立运作
//
// 3. 子包的形式取消, 都使用消息形式
//    正常的消息(s2g,g2s)形式就是 msg len tgid
//    当遇到本次消息的tgid和上一次一样(已经写入了一个消息), 改写上一个消息的tgid,
//    改写规则是: pid 和sid 等位置都是0(低32位), 高32位用来保存整个连续消息的总长度和数量,
//    本次写入的消息依然具有tgid, 但是第3个开始就没有tgid, 节省tgid, 同时节省了
//    消息处理的时候对消息的无效访问(大部分都是转发),
//    缺点是比较绕, 数据杂糅在一起
//
//    接到后, 解析消息, 先要看 tgid, 全0表示s给g的消息, 低32位0表示连续给tgid的消息, 这里的tgid可能是0
//    这个东东做完后, 基本上框架结束了
//
//    这个功能叫做网络消息包(有连续消息压缩功能),
//    这种形式就是被动消息连接, 主动的消息连接可能会打断消息先后, 造成游戏中一些时序操作出现问题,
//
//    先做成一个PacketWriter变异版本, 再封装为一个消息包处理者, 再封装为消息包处理器, 同时为多个目标进行消息包封装,
//    最后具备发送消息包给对应目标的能力(发送给不同Session)

type SGPacketWriter struct {
	Stream                  // 数据流
	ToTgid           uint64 // 投递目标Tgid
	PacketWriterType uint16 // 会话类型
	ToMailId         uint16 // 会话邮箱
	MsgCount         uint16 // 包内消息总数(包括尾随消息)
	PacketLen        uint16 // 包内数据总长度(所有数据)
	LastMsgPos       uint16 // 最近一个完整消息终止位置
	LastBeginMsgPos  uint16 // 最近一个完整消息开始位置
	CurrMsgId        uint16 // 当前消息Id
	SubHeaderPos     uint16 // 尾随头Pos
	SubMasterPos     uint16 // 尾随主体Pos
	SubMasterMsgId   uint16 // 尾随主体Id
	SubCount         uint16 // 尾随消息数
	SubLen           uint16 // 尾随数据长度
	SubTgid          uint16 // 尾随消息目标Id
	Subbing          bool   // 尾随中
}

func (this *SGPacketWriter) Init() {

}

func (this *SGPacketWriter) SetSubTgid(id uint64) bool {
	if id == this.SubTgid {
		if !this.Subbing {
			this.SubHeaderPos = this.Pos
			this.SubMasterPos = this.LastBeginMsgPos
			this.SubCount = 0
			this.SubLen = 0
			this.Subbing = true
			this.SubMasterMsgId = this.CurrMsgId
			// 让出尾随消息头

			if t.Pos+subMsgHeaderSize < t.MaxLen {
				this.Pos = this.Pos + subMsgHeaderSize
				return
			}

			return false
		}
	} else {
		if this.Subbing {
			// 原来是尾随, 现在要结束尾随
			if this.SubCount > 0 {
				// 1. 第一个消息的消息Id改写
				old_pos := this.Pos
				this.Pos = this.SubMasterPos + pckTgidSize
				this.WriteUint16(this.SubMasterMsgId + pckMsgExtern)
				this.Pos = old_pos
				// 2. 尾随头写入
				old_pos = this.SubHeaderPos
				this.Pos = this.SubHeaderPos
				this.WriteUint16(this.SubLen)
				this.WriteUint16(this.SubCount)
				this.Pos = old_pos
				// 3. 结束掉尾随标记
			}
			this.Subbing = false
		}

		this.SubTgid = id
		this.Subbing = false
	}

	return true
}

func (this *SGPacketWriter) WriteMsgId(id uint16) {

	if t.Pos+msgHeaderSize < t.MaxLen {
		this.MsgID = id
		this.CurrMsgId = id
		this.Pos = this.Pos + msgHeaderSize
		return
	}

	panic(errors.New("SGPacketWriter:WriteMsgId no long"))
}

// 写入一个消息
func (this *SGPacketWriter) WriteMsgOver() {
	// // 当前长度
	// msg_sum_len := uint32(this.Pos - this.LastMsgPos)

	// old_pos := this.Pos
	// this.Pos = this.LastMsgPos
	// this.WriteUint32((uint32(this.MsgID)<<16 | msg_sum_len))
	// this.Pos = old_pos
	// this.LastMsgPos = old_pos
	// this.Count++
}
