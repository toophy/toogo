package toogo

import (
	"strconv"
)

const (
	MaxNetMsgId = 30000
)

func (this *Thread) RegistNetMsg(id uint16, f NetMsgFunc) {
	this.netMsgProc[id] = f
}

func (this *Thread) RegistNetMsgDefault(f NetMsgDefaultFunc) {
	this.netDefault = f
}

// 响应网络消息包
func (this *Thread) procC2GNetPacket(m *Tmsg_packet) (ret bool) {

	errMsg := ""

	defer func() {
		if !ret {
			if len(errMsg) > 0 {
				this.LogWarn("Thread::procC2GNetPacket:" + errMsg)
			}
			this.self.On_packetError(m.SessionId)
		}
	}()

	defer RecoverCommon(this.threadId, "Thread::procC2GNetPacket:")

	this.packetReader.InitReader(m.Data, uint16(m.Count))
	this.packetReader.LinkTgid = m.Tgid

	for i := uint16(0); i < this.packetReader.Count; i++ {
		old_pos := this.packetReader.GetPos()
		msg_len, errLen := this.packetReader.XReadUint16()
		msg_id, errId := this.packetReader.XReadUint16()
		this.packetReader.PreReadMsg(msg_id, msg_len, old_pos)

		if !errLen || !errId {
			errMsg = "读取消息头失败"
			return
		}

		if msg_len < msgHeaderSize || uint64(msg_len) > this.packetReader.GetMaxLen()-old_pos {
			errMsg = "消息长度无效"
			return
		}

		if msg_id >= this.netMsgMaxId {
			errMsg = "消息ID无效"
			return
		}

		fc := this.netMsgProc[msg_id]
		if fc != nil {
			if !fc(&this.packetReader, m.SessionId) {
				errMsg = "读取消息体失败"
				return
			}
		} else {
			if this.netDefault != nil {
				if !this.netDefault(msg_id, &this.packetReader, m.SessionId) {
					errMsg = "读取消息体失败"
					return
				}
			} else {
				errMsg = "消息没有对应处理函数:" + strconv.Itoa(int(msg_id))
				return
			}
		}

		this.packetReader.Seek(old_pos + uint64(msg_len))
	}

	ret = true
	return
}

// 响应SS网络消息包
// 这个消息包, 里面是子消息包
func (this *Thread) procS2GNetPacketEx(m *Tmsg_packet) (ret bool) {

	errMsg := ""

	defer func() {
		if !ret {
			if len(errMsg) > 0 {
				this.LogWarn("Thread::procS2GNetPacketEx:" + errMsg)
			}
			this.self.On_packetError(m.SessionId)
		}
	}()

	defer RecoverCommon(this.threadId, "Thread::procS2GNetPacketEx:")

	this.packetReader.InitReader(m.Data, uint16(m.Count))

	subMessages := false
	var errLen bool
	var errId bool
	var errTgid bool
	var msg_len uint16
	var msg_id uint16
	var old_pos uint64
	var targetTgid uint64
	var subCount uint16
	var subMaxCount uint16
	// 包一层消息包
	//
	for i := uint16(0); i < m.Count; i++ {
		old_pos = this.packetReader.GetPos()
		if !subMessages {
			targetTgid, errTgid = this.packetReader.XReadUint64()
			if !errTgid {
				errMsg = "读取Sub消息头失败"
				return
			}
			this.packetReader.LinkTgid = targetTgid
			if subCount != subMaxCount {
				errMsg = "Sub消息数量不正确"
				return
			}
			subCount = 0
			subMaxCount = 0
		} else {
			subCount++
		}

		msg_len, errLen = this.packetReader.XReadUint16()
		msg_id, errId = this.packetReader.XReadUint16()
		this.packetReader.PreReadMsg(msg_id, msg_len, old_pos)

		if !errLen || !errId {
			errMsg = "读取消息头失败"
			return
		}

		if msg_len < msgHeaderSize || uint64(msg_len) > this.packetReader.GetMaxLen()-old_pos {
			errMsg = "SG消息长度无效:" + strconv.Itoa(int(msg_len))
			return
		}

		if msg_id > MaxNetMsgId {
			// 连续包
			if !subMessages {
				msg_id = msg_id - MaxNetMsgId
				subMessages = true
			} else {
				errMsg = "Sub包内不能嵌入Sub包"
				return
			}
		}

		if msg_id >= this.netMsgMaxId {
			errMsg = "消息ID无效"
			return
		}

		fc := this.netMsgProc[msg_id]
		if fc != nil {
			if !fc(&this.packetReader, m.SessionId) {
				errMsg = "读取消息体失败"
				return
			}
		} else {
			if this.netDefault != nil {
				if !this.netDefault(msg_id, &this.packetReader, m.SessionId) {
					errMsg = "读取消息体失败"
					return
				}
			} else {
				errMsg = "消息没有对应处理函数:" + strconv.Itoa(int(msg_id))
				return
			}
		}

		this.packetReader.Seek(old_pos + uint64(msg_len))
	}

	ret = true
	return
}
