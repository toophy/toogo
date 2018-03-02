package toogo

import (
	"fmt"
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
	funcLogFlag := "Thread::procC2GNetPacket:"

	defer func() {
		if !ret {
			if len(errMsg) > 0 {
				this.LogWarn("Thread::procC2GNetPacket:" + errMsg)
			}
			this.self.On_packetError(m.SessionId)
		}
	}()

	defer RecoverCommon(this.threadId, "Thread::procC2GNetPacket:")()

	this.packetReader.InitReader(m.Data, uint16(m.Count))
	this.packetReader.LinkTgid = m.Tgid

	for i := uint16(0); i < this.packetReader.Count; i++ {
		old_pos := this.packetReader.GetPos()
		msg_len, errLen := this.packetReader.XReadUint16()
		msg_id, errId := this.packetReader.XReadUint16()
		this.packetReader.PreReadMsg(msg_id, msg_len, old_pos)

		if !errLen || !errId {
			errMsg = funcLogFlag + "读取消息头失败"
			return
		}

		if msg_len < msgHeaderSize || uint64(msg_len) > this.packetReader.GetMaxLen()-old_pos {
			errMsg = fmt.Sprintf("%s消息长度无效%d", funcLogFlag, msg_len)
			return
		}

		errMsg = this.procReadMsgAndExec("C2GNetPacket", msg_id, m)
		if len(errMsg) > 0 {
			return
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
	funcLogFlag := "Thread::procS2GNetPacket:"

	defer func() {
		if !ret {
			if len(errMsg) > 0 {
				this.LogWarn(funcLogFlag + errMsg)
			}
			this.self.On_packetError(m.SessionId)
		}
	}()

	defer RecoverCommon(this.threadId, funcLogFlag)()

	this.packetReader.InitReader(m.Data, uint16(m.Count))

	var targetTgid uint64
	var errTgid bool
	var subPacketLen uint16
	var subPacketCount uint16
	// 包一层消息包
	//
	for i := uint16(0); i < m.Count; i++ {
		old_pos := this.packetReader.GetPos()

		msg_len, errLen := this.packetReader.XReadUint16()
		msg_id, errId := this.packetReader.XReadUint16()
		this.packetReader.PreReadMsg(msg_id, msg_len, old_pos)

		if !errLen || !errId {
			errMsg = funcLogFlag + "读取消息头失败"
			return
		}

		if msg_len < msgHeaderSize || uint64(msg_len) > this.packetReader.GetMaxLen()-old_pos {
			errMsg = fmt.Sprintf("%s消息长度无效%d", funcLogFlag, msg_len)
			return
		}

		if msg_id < pckMsgExtern {
			errMsg = this.procReadMsgAndExec("S2GNetPacketA", msg_id, m)
			if len(errMsg) > 0 {
				return
			}
			this.packetReader.Seek(old_pos + uint64(msg_len))
		} else if msg_id < pckMsgExtern2 {
			// Server请求Gate服中转的一条消息
			msg_id = msg_id - pckMsgExtern
			targetTgid, errTgid = this.packetReader.XReadUint64()
			if !errTgid {
				errMsg = fmt.Sprintf("S2GNetPacketB:读取消息来源Tgid失败 MID(%d)", msg_id)
				return
			}
			msg_body := this.packetReader.GetPos()
			this.packetReader.Seek(old_pos + uint64(msg_len))
			// 接下来转移这个消息给对应的Session
			// msg_body 到 当前位置 , 都转移

			targetSessionId := GetSessionIdByTgid(targetTgid)
			if targetSessionId != 0 {
				p := NewPacket(uint32(msg_len)*2, targetSessionId)

				if p != nil {
					if Tgid_is_Rid(targetTgid) {
						fmt.Printf("%-v\n", this.packetReader.GetData()[:20])
						fmt.Printf("%d,%d,%d,%d", msg_id, msg_body, msg_len, uint64(msg_len-msgHeaderSize-pckTgidSize))
						p.CopyFromPacketReaderEx(1, msg_id, &this.packetReader, msg_body, uint64(msg_len-msgHeaderSize-pckTgidSize))
					} else {
						p.CopyFromPacketReaderEx2(1, msg_id, m.Tgid, &this.packetReader, msg_body, uint64(msg_len-msgHeaderSize-pckTgidSize))
					}
					SendPacket(p)
				}
			}
		} else if msg_id < pckMsgExtern3 {
			// Gate服中转的一串消息
			msg_id = msg_id - pckMsgExtern2
			targetTgid, errTgid = this.packetReader.XReadUint64()
			if !errTgid {
				errMsg = fmt.Sprintf("S2GNetPacketC:读取消息来源Tgid失败 MID(%d)", msg_id)
				return
			}
			msg_body := this.packetReader.GetPos()

			// 第一个消息处理完毕, 接下来是消息串的头部
			subPacketLen = this.packetReader.ReadUint16()
			subPacketCount = this.packetReader.ReadUint16()

			this.packetReader.Seek(old_pos + uint64(msg_len))

			sugMsgBegin := this.packetReader.GetPos()

			// 接下来转移这串消息给对应的Session
			// msg_body 到 当前位置 , 都转移

			targetSessionId := GetSessionIdByTgid(targetTgid)
			if targetSessionId != 0 {
				p := NewPacket(uint32(msg_len)*2, targetSessionId)

				if p != nil {
					if Tgid_is_Rid(targetTgid) {
						p.CopyFromPacketReaderEx(1, msg_id, &this.packetReader, msg_body, uint64(msg_len-msgHeaderSize-pckTgidSize-subMsgHeaderSize))
						p.CopyFromPacketReader(subPacketCount-1, &this.packetReader, sugMsgBegin, uint64(subPacketLen-msg_len))
					} else {
						p.CopyFromPacketReaderEx2(subPacketCount, msg_id, m.Tgid, &this.packetReader, msg_body, uint64(subPacketLen-msgHeaderSize-pckTgidSize))
					}
					SendPacket(p)
				}
			}

		} else {
			errMsg = fmt.Sprintf("S2GNetPacketD:消息ID无效 MID(%d)", msg_id)
			return
		}
	}

	ret = true
	return
}

// 响应SS网络消息包
// 这个消息包, 里面是子消息包
func (this *Thread) procG2SNetPacket(m *Tmsg_packet) (ret bool) {

	errMsg := ""
	funcLogFlag := "Thread::procG2SNetPacket:"

	defer func() {
		if !ret {
			if len(errMsg) > 0 {
				this.LogWarn(funcLogFlag + errMsg)
			}
			this.self.On_packetError(m.SessionId)
		}
	}()

	defer RecoverCommon(this.threadId, funcLogFlag)()

	this.packetReader.InitReader(m.Data, uint16(m.Count))

	var targetTgid uint64
	var errTgid bool
	/*var subPacketLen uint16*/
	var subPacketCount uint16
	var subPacketReaderCount uint16
	// 包一层消息包
	//
	for i := uint16(0); i < m.Count; i++ {
		old_pos := this.packetReader.GetPos()

		msg_len, errLen := this.packetReader.XReadUint16()
		msg_id, errId := this.packetReader.XReadUint16()
		this.packetReader.PreReadMsg(msg_id, msg_len, old_pos)

		if !errLen || !errId {
			errMsg = "读取消息头失败"
			return
		}

		if msg_len < msgHeaderSize || uint64(msg_len) > this.packetReader.GetMaxLen()-old_pos {
			errMsg = fmt.Sprintf("消息长度无效%d>%d,maxLen=%d, MID(%d)", msg_len,
				this.packetReader.GetMaxLen()-old_pos, this.packetReader.GetMaxLen(), msg_id)
			return
		}

		if msg_id < pckMsgExtern {
			errMsg = this.procReadMsgAndExec("G2SNetPacketA", msg_id, m)
			if len(errMsg) > 0 {
				return
			}
			this.packetReader.Seek(old_pos + uint64(msg_len))

			if this.packetReader.LinkTgid != 0 {
				// Gate服中转的尾随消息, 处理尾随消息读取数量
				subPacketReaderCount++
				if subPacketReaderCount >= subPacketCount {
					this.packetReader.LinkTgid = 0
					/*subPacketLen = 0*/
					subPacketCount = 0
					subPacketReaderCount = 0
				}
			}
		} else if msg_id < pckMsgExtern2 {
			// Gate服中转的一条消息
			msg_id = msg_id - pckMsgExtern
			targetTgid, errTgid = this.packetReader.XReadUint64()
			if !errTgid {
				errMsg = fmt.Sprintf("G2SNetPacketB:读取消息来源Tgid失败 MID(%d)", msg_id)
				return
			}
			//
			this.packetReader.LinkTgid = targetTgid
			//
			errMsg = this.procReadMsgAndExec("G2SNetPacketB", msg_id, m)
			if len(errMsg) > 0 {
				return
			}
			this.packetReader.Seek(old_pos + uint64(msg_len))

			this.packetReader.LinkTgid = 0
		} else if msg_id < pckMsgExtern3 {
			// Gate服中转的一串消息
			msg_id = msg_id - pckMsgExtern2
			targetTgid, errTgid = this.packetReader.XReadUint64()
			if !errTgid {
				errMsg = fmt.Sprintf("G2SNetPacketC:读取消息来源Tgid失败 MID(%d)", msg_id)
				return
			}
			//
			this.packetReader.LinkTgid = targetTgid
			//
			errMsg = this.procReadMsgAndExec("G2SNetPacketC", msg_id, m)
			if len(errMsg) > 0 {
				return
			}

			// 第一个消息处理完毕, 接下来是消息串的头部
			/*subPacketLen = */ this.packetReader.ReadUint16()
			subPacketCount = this.packetReader.ReadUint16()

			this.packetReader.Seek(old_pos + uint64(msg_len))

			subPacketReaderCount = 1
		} else {
			errMsg = fmt.Sprintf("G2SNetPacketD:消息ID无效 MID(%d)", msg_id)
			return
		}
	}

	ret = true
	return
}

// 响应SS网络消息包
// 这个消息包, 里面是子消息包
func (this *Thread) procReadMsgAndExec(flag string, msg_id uint16, m *Tmsg_packet) string {

	if msg_id >= this.netMsgMaxId {
		return fmt.Sprintf("%s:消息ID无效A MID(%d)", flag, msg_id)
	}
	fc := this.netMsgProc[msg_id]
	if fc != nil {
		if !fc(&this.packetReader, m.SessionId) {
			return fmt.Sprintf("%s:读取消息体失败A MID(%d)", flag, msg_id)
		}
	} else {
		if this.netDefault != nil {
			if !this.netDefault(msg_id, &this.packetReader, m.SessionId) {
				return fmt.Sprintf("%s:默认读取消息体失败A MID(%d)", flag, msg_id)
			}
		} else {
			return fmt.Sprintf("%s:消息没有对应处理函数A MID(%d)", flag, msg_id)
		}
	}

	return ""
}
