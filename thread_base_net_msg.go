package toogo

import (
	"strconv"
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

	defer RecoverCommon(this.id, "Thread::procC2GNetPacket:")

	this.packetReader.InitReader(m.Data, uint16(m.Count))

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

	this.packetReader.InitReader(m.Data, uint16(m.Count))

	// 包一层消息包
	//
	for i := uint16(0); i < m.Count; i++ {
		// 子消息包头
		old_packet_pos := this.packetReader.GetPos()
		packet_len, errPLen := this.packetReader.XReadUint24()
		msg_count, errPCount := this.packetReader.XReadUint16()
		targetTgid, errTgid := this.packetReader.XReadUint64()
		if !errPLen || !errPCount || !errTgid || packet_len <= 0 {
			errMsg = "读取消息包头失败"
			return
		}
		println("game_session0")
		if targetTgid == 0 {
			// for gate message
			for k := uint16(0); k < msg_count; k++ {
				old_pos := this.packetReader.GetPos()
				msg_len, errLen := this.packetReader.XReadUint16()
				msg_id, errId := this.packetReader.XReadUint16()
				this.packetReader.PreReadMsg(msg_id, msg_len, old_pos)

				if !errLen || !errId {
					errMsg = "读取消息头失败"
					return
				}

				if msg_len < msgHeaderSize || uint64(msg_len) > this.packetReader.GetMaxLen()-old_pos {
					errMsg = "SG消息长度无效"
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
		} else if Tgid_is_Sid(targetTgid) {

			game_session := GetSessionIdByTgid(targetTgid)
			println("game_session1", game_session, targetTgid, i, old_packet_pos, packet_len, msg_count)
			if game_session != 0 {
				println("game_session2", game_session)
				px := NewPacket(packet_len, game_session)
				if px != nil {
					px.CopyFromPacketReader(&this.packetReader, old_packet_pos+13, uint64(packet_len-13))
					px.Count = msg_count
					px.Tgid = m.Tgid
					SendPacket(px)
				}
			}
		} else if Tgid_is_Rid(targetTgid) {
			game_session := GetSessionIdByTgid(targetTgid)
			println("game_session10", game_session, targetTgid, i, old_packet_pos, packet_len, msg_count)
			if game_session != 0 {
				println("game_session12", game_session)
				px := NewPacket(packet_len, game_session)
				if px != nil {
					px.CopyFromPacketReader(&this.packetReader, old_packet_pos+13, uint64(packet_len-13))
					px.Count = msg_count
					px.Tgid = m.Tgid
					SendPacket(px)
				}
			}
		} else {
			// 暂时不支持这些, 丢弃吧
		}

		this.packetReader.Seek(old_packet_pos + uint64(packet_len))
	}

	ret = true
	return
}
