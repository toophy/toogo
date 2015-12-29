package toogo

import (
	"strconv"
)

func (this *Thread) RegistNetMsg(id uint16, f NetMsgFunc) {
	this.netMsgProc[id] = f
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

	defer RecoverCommon(this.id, "Thread::procC2GNetPacket:")

	p := new(PacketReader)
	p.InitReader(m.Data, uint16(m.Count))

	for i := uint16(0); i < p.Count; i++ {
		old_pos := p.GetPos()
		msg_len, errLen := p.XReadUint16()
		msg_id, errId := p.XReadUint16()
		p.PreReadMsg(msg_id, msg_len, old_pos)

		if !errLen || !errId {
			errMsg = "读取消息头失败"
			return
		}

		if msg_len < msgHeaderSize || uint64(msg_len) > p.GetMaxLen()-old_pos {
			errMsg = "消息长度无效"
			return
		}

		if msg_id >= this.netMsgMaxId {
			errMsg = "消息ID无效"
			return
		}

		fc := this.netMsgProc[msg_id]
		if fc == nil {
			errMsg = "消息没有对应处理函数:" + strconv.Itoa(int(msg_id))
			return
		}

		if !fc(p, m.SessionId) {
			errMsg = "读取消息体失败"
			return
		}

		p.Seek(old_pos + uint64(msg_len))
	}

	ret = true
	return
}

// 响应SS网络消息包
// 这个消息包, 里面是子消息包
func (this *Thread) procS2GNetPacket(m *Tmsg_packet) (ret bool) {

	errMsg := ""

	defer func() {
		if !ret {
			if len(errMsg) > 0 {
				this.LogWarn("Thread::procS2GNetPacket:" + errMsg)
			}
			this.self.On_packetError(m.SessionId)
		}
	}()

	defer RecoverCommon(this.id, "Thread::procS2GNetPacket:")

	p := new(PacketReader)
	p.InitReader(m.Data, uint16(m.Count))

	// 包一层消息包
	//
	for i := uint16(0); i < m.Count; i++ {
		// 子消息包头
		packet_len, errPLen := p.XReadUint24()
		msg_count, errPCount := p.XReadUint16()
		flag, errFlag := p.XReadUint64()
		if !errPLen || !errPCount || !errFlag || packet_len <= 0 {
			errMsg = "读取消息包头失败"
			return
		}

		if flag == 0 {
			// for gate message
			for k := uint16(0); k < msg_count; k++ {
				old_pos := p.GetPos()
				msg_len, errLen := p.XReadUint16()
				msg_id, errId := p.XReadUint16()
				p.PreReadMsg(msg_id, msg_len, old_pos)

				if !errLen || !errId {
					errMsg = "读取消息头失败"
					return
				}

				if msg_len < msgHeaderSize || uint64(msg_len) > p.GetMaxLen()-old_pos {
					errMsg = "SG消息长度无效"
					return
				}

				if msg_id >= this.netMsgMaxId {
					errMsg = "消息ID无效"
					return
				}

				fc := this.netMsgProc[msg_id]
				if fc == nil {
					errMsg = "消息没有对应处理函数:" + strconv.Itoa(int(msg_id))
					return
				}

				if !fc(p, m.SessionId) {
					errMsg = "读取消息体失败"
					return
				}

				p.Seek(old_pos + uint64(msg_len))
			}
		} else {
			// client
			// game server(1...n)
			// mail
			// chat
			// ...
		}

	}

	ret = true
	return
}
