package toogo

import (
	"errors"
)

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
	Stream                 // 数据流
	LastMsgPos      uint64 // 最近一个消息终止位置
	ToMailId        uint32 // 会话邮箱
	MsgCount        uint16 // 包内消息总数(包括尾随消息)
	MsgID           uint16 // 当前消息ID
	WritePacketType uint16 // 会话类型
	ToTgid          uint64 // 投递目标Tgid
	lastMsgPos      uint64 // 最近一个完整消息终止位置
	lastBeginMsgPos uint64 // 最近一个完整消息开始位置
	subHeaderSize   uint64 // 尾随头Pos
	subMasterPos    uint64 // 尾随主体Pos
	subMasterMsgId  uint16 // 尾随主体Id
	subCount        uint16 // 尾随消息数
	subLen          uint64 // 尾随数据长度
	subTgid         uint64 // 尾随消息目标Id
	subbing         bool   // 尾随中
	initted         bool   // 被初始化过
	noTgidHeader    bool   // 没有tgid头的网络封包, 用于客户端通信
}

func (this *PacketWriter) InitWriter(d []byte, pckType uint16, mailId uint32) {
	this.Init(d)
	this.WritePacketType = pckType
	headerSize := getHeaderSize(this.WritePacketType)
	this.lastMsgPos = uint64(headerSize)
	this.Pos = uint64(headerSize)
	this.ToMailId = mailId

	if this.WritePacketType == SessionPacket_C2G || this.WritePacketType == SessionPacket_G2C {
		this.noTgidHeader = true
	} else {
		this.noTgidHeader = false
	}

	this.initted = true
}

func (this *PacketWriter) Reset(d []byte, pckType uint16, mailId uint32) {
	this.Init(d)
	this.WritePacketType = pckType
	headerSize := getHeaderSize(this.WritePacketType)
	this.lastMsgPos = uint64(headerSize)
	this.Pos = uint64(headerSize)
	this.ToMailId = mailId
	this.subbing = false
	if this.WritePacketType == SessionPacket_C2G || this.WritePacketType == SessionPacket_G2C {
		this.noTgidHeader = true
	} else {
		this.noTgidHeader = false
	}

	if this.initted {
		this.ToTgid = 0
		this.MsgCount = 0
		this.lastMsgPos = 0
		this.lastBeginMsgPos = 0
		this.MsgID = 0
		this.subHeaderSize = 0
		this.subMasterPos = 0
		this.subMasterMsgId = 0
		this.subCount = 0
		this.subLen = 0
		this.subTgid = 0
	} else {
		this.initted = true
	}
}

func (this *PacketWriter) SetsubTgid(id uint64) {
	if this.noTgidHeader {
		return
	}

	if id == this.subTgid {
		if !this.subbing {
			this.subHeaderSize = this.Pos
			this.subMasterPos = this.lastBeginMsgPos
			this.subCount = 0
			this.subLen = 0
			this.subbing = true
			this.subMasterMsgId = this.MsgID
			// 让出尾随消息头

			if this.Pos+subMsgHeaderSize < this.MaxLen {
				this.Pos = this.Pos + subMsgHeaderSize
				this.WriteUint64(id)
			} else {
				panic(errors.New("PacketWriter:SetsubTgid no long"))
			}
		}
	} else {
		this.subMsgOver()
		this.subTgid = id
	}
}

// 写入消息ID
func (this *PacketWriter) WriteMsgId(id uint16) {
	if this.noTgidHeader {
		if this.Pos+msgHeaderSize < this.MaxLen {
			this.MsgID = id
			this.Pos = this.Pos + msgHeaderSize
			return
		}
	} else {
		if this.Pos+msgHeaderSize < this.MaxLen {
			this.MsgID = id
			this.lastBeginMsgPos = this.Pos
			this.lastMsgPos = this.Pos
			this.Pos = this.Pos + msgHeaderSize
			return
		}
	}
	panic(errors.New("SGPacketWriter:WriteMsgId no long"))
}

func (this *PacketWriter) subMsgOver() {
	if this.subbing {
		// 原来是尾随, 现在要结束尾随
		if this.subCount > 0 {
			// 1. 第一个消息的消息Id改写
			old_pos := this.Pos
			this.Pos = this.subMasterPos + pckTgidSize
			this.WriteUint16(this.subMasterMsgId + pckMsgExtern)
			// 2. 尾随头写入
			old_pos = this.subHeaderSize
			this.Pos = this.subHeaderSize
			this.WriteUint16(uint16(this.subLen))
			this.WriteUint16(this.subCount)
			this.Pos = old_pos
			// 3. 结束掉尾随标记
		}
		this.subbing = false
		this.subLen = 0
		this.subCount = 0
	}
}

// 写入一个消息
func (this *PacketWriter) WriteMsgOver() {
	// 当前长度
	msg_sum_len := uint32(this.Pos - this.lastBeginMsgPos)

	old_pos := this.Pos
	this.Pos = this.lastBeginMsgPos
	this.WriteUint32((uint32(this.MsgID)<<16 | msg_sum_len))
	this.Pos = old_pos
	this.lastMsgPos = old_pos
	this.MsgCount++

	if !this.noTgidHeader {
		if this.subbing {
			this.subCount++
			this.subLen += uint64(msg_sum_len)
		}
	}
}

// 结束一个封包
func (this *PacketWriter) PacketWriteOver() {

	this.subMsgOver()

	packet_len := this.Pos
	token := uint32(0)

	old_pos := this.Pos
	this.Pos = 0

	switch this.WritePacketType {
	case SessionPacket_C2G:
		this.WriteUint16(uint16(packet_len))
		this.WriteUint8(uint8(token))
		this.WriteUint8(uint8(this.MsgCount))
	case SessionPacket_G2C:
		this.WriteUint16(uint16(packet_len))
		this.WriteUint16(this.MsgCount)
	case SessionPacket_G2S:
		this.WriteUint24(uint32(packet_len))
		this.WriteUint16(this.MsgCount)
	case SessionPacket_S2G:
		this.WriteUint24(uint32(packet_len))
		this.WriteUint16(this.MsgCount)
	}

	this.Pos = old_pos
}

// 拷贝一个完整消息
func (this *PacketWriter) CopyMsg(d []byte, dLen uint64) bool {
	defer RecoverCommon(0, "PacketWriter::CopyMsg")

	this.WriteDataEx(d, dLen)
	this.lastMsgPos = this.lastMsgPos + dLen
	this.MsgCount++

	return true
}

// 拷贝定长消息
func (this *PacketWriter) CopyFromPacketReader(r *PacketReader, pos uint64, dLen uint64) bool {
	defer RecoverCommon(0, "PacketWriter::CopyFromPacketReader")

	this.WriteDataEx(r.Data[pos:pos+dLen], dLen)
	this.lastMsgPos = this.lastMsgPos + dLen

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
