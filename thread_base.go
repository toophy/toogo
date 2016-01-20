package toogo

import (
	"errors"
	"time"
)

// 线程空接口, 继承Thread结构, 必须实现下列接口
type IThreadChild interface {
	On_firstRun()                    // -- 只允许thread调用 : 首次运行(在 on_run 前面)
	On_preRun()                      // -- 只允许thread调用 : 线程最先运行部分
	On_run()                         // -- 只允许thread调用 : 线程运行部分
	On_end()                         // -- 只允许thread调用 : 线程结束回调
	On_netEvent(m *Tmsg_net) bool    // -- 响应网络事件
	On_registNetMsg()                // -- 注册网络消息的响应函数
	On_packetError(sessionId uint64) // -- 当网络消息包解析出现问题, 如何处理?
}

// 线程接口
type IThread interface {
	ILogOs
	IThreadChild
	IEventOs
	Init_thread(IThread, uint32, string, uint16, int64, uint64) error // 初始化线程
	Run_thread()                                                      // 运行线程
	Get_thread_id() uint32                                            // 获取线程ID
	Get_thread_name() string                                          // 获取线程名称
	Pre_close_thread()                                                // -- 只允许thread调用 : 预备关闭线程
	RegistNetMsg(id uint16, f NetMsgFunc)                             // -- 注册网络消息处理函数

	// toogo库私有接口
	procC2GNetPacket(m *Tmsg_packet) bool   // -- 响应网络消息包 Session是CG类型
	procS2GNetPacketEx(m *Tmsg_packet) bool // -- 响应网络消息包 Session是SG类型
}

const (
	updateCurrTimeCount = 32 // 刷新时间戳变更上线
	Tid_master          = 1  // 主线程
	Tid_last            = 16 // 最后一条重要线程
)

// 消息函数类型
type NetMsgFunc func(p *PacketReader, sessionId uint64) bool

// 消息函数类型
type NetMsgDefaultFunc func(msg_id uint16, p *PacketReader, sessionId uint64) bool

// 线程基本功能
type Thread struct {
	LogOs                                    // 线程日志
	EventOs                                  // 线程事件
	name                string               // 线程名称
	heart_time          int64                // 心跳时间(毫秒)
	start_time          int64                // 线程开启时间戳
	last_time           int64                // 最近一次线程运行时间戳
	curr_time           int64                // 当前时间戳(毫秒)
	get_curr_time_count int64                // 索取当前时间戳次数
	heart_rate          float64              // 本次心跳比率
	pre_stop            bool                 // 预备停止
	first_run           bool                 // 线程首次运行
	netDefault          NetMsgDefaultFunc    // 默认网络消息处理函数
	netMsgProc          []NetMsgFunc         // 网络消息函数注册表
	netMsgMaxId         uint16               // 最大网络消息ID
	packetReader        PacketReader         // 网络包读者
	threadMsgs          [Tid_last]*DListNode // 保存将要发给其他线程的事件(消息)
}

// 初始化线程(必须调用)
// usage : Init_thread(Tid_master, "主线程", 100)
// self       : 继承者自身指针
// id         : 申请的线程ID
// name       : 线程别名
// max_msg_id : 最大网络消息ID
// heart_time : 心跳间隔(毫秒)
// lay1_time  : 第一层支持的时间长度(毫秒)
func (this *Thread) Init_thread(self IThread, id uint32, name string, max_msg_id uint16, heart_time int64, lay1_time uint64) error {
	if id < Tid_master || id >= Tid_last {
		return errors.New("[E] 线程ID超出范围 [Tid_master,Tid_last]")
	}
	if self == nil {
		return errors.New("[E] 线程自身指针不能为nil")
	}

	this.threadId = id
	this.name = name
	this.heart_time = heart_time * int64(time.Millisecond)
	this.start_time = time.Now().UnixNano()
	this.last_time = this.start_time

	// 设置当前时间戳(毫秒)
	this.get_curr_time_count = 1
	this.curr_time = this.last_time / int64(time.Millisecond)

	this.heart_rate = 1.0
	this.first_run = true

	for i := 0; i < Tid_last; i++ {
		this.threadMsgs[i] = new(DListNode)
		this.threadMsgs[i].Init(nil)
	}

	errLog := this.InitLog(this.Get_thread_id())
	if errLog != nil {
		return errLog
	}

	errEventOs := this.InitEventOs(self, lay1_time)
	if errEventOs != nil {
		return errEventOs
	}

	if max_msg_id < 1 {
		panic("网络消息注册表不能为空")
	}
	if max_msg_id > 60000 {
		panic("网络消息注册表最大不能超过60000")
	}

	this.netMsgMaxId = max_msg_id
	this.netMsgProc = make([]NetMsgFunc, this.netMsgMaxId)
	this.self.On_registNetMsg()

	return nil
}

// 运行线程
func (this *Thread) Run_thread() {
	// 计算心跳误差值, 决定心跳滴答(小数), heart_time, last_time, heart_rate
	// 处理线程间接收消息, 分配到水表定时器
	// 执行水表定时器
	EnterThread()
	go func() {
		defer LeaveThread()
		defer RecoverCommon(this.threadId, "Thread::Run_thread:")

		this.start_time = time.Now().UnixNano()
		this.last_time = this.start_time
		next_time := time.Duration(this.heart_time)
		run_time := int64(0)

		this.UpdateLogTimeHeader()

		this.self.On_firstRun()

		for {

			time.Sleep(next_time)

			this.UpdateLogTimeHeader()

			this.last_time = time.Now().UnixNano()
			// 设置当前时间戳(毫秒)
			this.get_curr_time_count = 1
			this.curr_time = this.last_time / int64(time.Millisecond)

			this.FlushToFile(this.curr_time)

			this.self.On_preRun()

			this.runThreadMsg()
			this.runEvents((this.last_time - this.start_time) / int64(time.Millisecond))
			this.self.On_run()

			this.SendThreadLog()

			for i := uint32(Tid_master); i < Tid_last; i++ {
				if !this.threadMsgs[i].IsEmpty() {
					GetThreadMsgs().PostMsg(i, this.threadMsgs[i])
				}
			}

			// 计算下一次运行的时间
			run_time = time.Now().UnixNano() - this.last_time
			if run_time >= this.heart_time {
				run_time = this.heart_time - 10*1000*1000
			} else if run_time < 0 {
				run_time = 0
			}

			next_time = time.Duration(this.heart_time - run_time)

			if this.pre_stop {
				// 是否有需要释放的对象?
				this.self.On_end()
				if this.is_master_thread() {
					this.log_FileHandle.Close()
				}
				break
			}
		}
	}()
}

// 返回线程编号
func (this *Thread) Get_thread_id() uint32 {
	return this.threadId
}

// 返回线程名称
func (this *Thread) Get_thread_name() string {
	return this.name
}

// 是世界线程
func (this *Thread) is_master_thread() bool {
	return this.threadId == Tid_master
}

// 预备关闭线程
func (this *Thread) Pre_close_thread() {
	this.pre_stop = true
}

// 投递线程间消息
func (this *Thread) PostThreadMsg(tid uint32, a IThreadMsg) bool {
	if tid == this.Get_thread_id() {
		this.LogWarn("PostThreadMsg dont post to self")
		return false
	}
	if tid >= Tid_master && tid < Tid_last {
		header := this.threadMsgs[tid]

		n := new(DListNode)
		if n == nil {
			this.LogError("PostThreadMsg newDlinkNode failed")
			return false
		}
		n.Init(a)

		old_pre := header.Pre

		header.Pre = n
		n.Next = header
		n.Pre = old_pre
		old_pre.Next = n

		return true
	}
	this.LogWarn("PostThreadMsg post msg failed")
	return false
}

// 接收并处理线程间消息
func (this *Thread) runThreadMsg() {

	header := DListNode{}
	header.Init(nil)

	GetThreadMsgs().GetMsg(this.Get_thread_id(), &header)

	for {
		n := header.Next
		if n.IsEmpty() {
			break
		}

		n.Data.(IThreadMsg).Exec(this.self)
		n.Pop()
	}
}

// 获取当前时间戳(毫秒)
func (this *Thread) GetCurrTime() int64 {
	this.get_curr_time_count++
	if this.get_curr_time_count > updateCurrTimeCount {
		this.get_curr_time_count = 1
		this.curr_time = time.Now().UnixNano() / int64(time.Millisecond)
	}

	return this.curr_time
}
