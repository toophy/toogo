package toogo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// 线程接口
type IThread interface {
	Init_thread(IThread, int32, string, int64, uint64) error // 初始化线程
	Run_thread()                                             // 运行线程
	Get_thread_id() int32                                    // 获取线程ID
	Get_thread_name() string                                 // 获取线程名称
	pre_close_thread()                                       // -- 只允许thread调用 : 预备关闭线程
	on_first_run()                                           // -- 只允许thread调用 : 首次运行(在 on_run 前面)
	on_pre_run()                                             // -- 只允许thread调用 : 线程最先运行部分
	on_run()                                                 // -- 只允许thread调用 : 线程运行部分
	on_end()                                                 // -- 只允许thread调用 : 线程结束回调

	PostEvent(a IEvent) bool     // 投递定时器事件
	GetEvent(name string) IEvent // 通过别名获取事件
	RemoveEvent(e IEvent)        // 删除事件, 只能操作本线程事件

	LogDebug(f string, v ...interface{}) // 线程日志 : 调试[D]级别日志
	LogInfo(f string, v ...interface{})  // 线程日志 : 信息[I]级别日志
	LogWarn(f string, v ...interface{})  // 线程日志 : 警告[W]级别日志
	LogError(f string, v ...interface{}) // 线程日志 : 错误[E]级别日志
	LogFatal(f string, v ...interface{}) // 线程日志 : 致命[F]级别日志

	sendThreadFreeNode()           // 发送线程间回收
	releaseDlinkNode(d *DListNode) // 释放节点
}

// 线程基本功能
type Thread struct {
	id                  uint32                // Id号
	name                string                // 线程名称
	heart_time          int64                 // 心跳时间(毫秒)
	start_time          int64                 // 线程开启时间戳
	last_time           int64                 // 最近一次线程运行时间戳
	curr_time           int64                 // 当前时间戳(毫秒)
	get_curr_time_count int64                 // 索取当前时间戳次数
	heart_rate          float64               // 本次心跳比率
	pre_stop            bool                  // 预备停止
	self                IThread               // 自己, 初始化之后, 不要操作
	first_run           bool                  // 线程首次运行
	evt_lay1            []DListNode           // 第一层事件池
	evt_lay2            map[uint64]*DListNode // 第二层事件池
	evt_names           map[string]IEvent     // 别名
	evt_lay1Size        uint64                // 第一层池容量
	evt_lay1Cursor      uint64                // 第一层游标
	evt_lastRunCount    uint64                // 最近一次运行次数
	evt_currRunCount    uint64                // 当前运行次数
	evt_threadMsg       [Tid_last]*DListNode  // 保存将要发给其他线程的事件(消息)
	log_Buffer          []byte                // 线程日志缓冲
	log_BufferLen       int                   // 线程日志缓冲长度
	log_TimeString      string                // 时间格式(精确到秒2015.08.13 16:33:00)
	log_Header          [LogMaxLevel]string   // 各级别日志头
	log_FileBuff        bytes.Buffer          // 日志总缓冲, Tid_world才会使用
	log_FileHandle      *os.File              // 日志文件, Tid_world才会使用
	log_FlushTime       int64                 // 日志文件最后写入时间
}

// 初始化线程(必须调用)
// usage : Init_thread(Tid_world, "主线程", 100)
func (this *Thread) Init_thread(self IThread, id uint32, name string, heart_time int64, lay1_time uint64) error {
	if id < Tid_world || id >= Tid_last {
		return errors.New("[E] 线程ID超出范围 [Tid_world,Tid_last]")
	}
	if self == nil {
		return errors.New("[E] 线程自身指针不能为nil")
	}

	if lay1_time < Evt_gap_time || lay1_time > Evt_lay1_time {
		return errors.New("[E] 第一层支持16毫秒到160000毫秒")
	}

	if len(this.evt_names) > 0 {
		return errors.New("[E] EventHome 已经初始化过")
	}

	this.id = id
	this.name = name
	this.heart_time = heart_time * int64(time.Millisecond)
	this.start_time = time.Now().UnixNano()
	this.last_time = this.start_time

	// 设置当前时间戳(毫秒)
	this.get_curr_time_count = 1
	this.curr_time = this.last_time / int64(time.Millisecond)

	this.heart_rate = 1.0
	this.self = self
	this.first_run = true

	// 初始化事件池
	this.evt_lay1Size = lay1_time >> Evt_gap_bit
	this.evt_lay1Cursor = 0
	this.evt_currRunCount = 1
	this.evt_lastRunCount = this.evt_currRunCount

	this.evt_lay1 = make([]DListNode, this.evt_lay1Size)
	this.evt_lay2 = make(map[uint64]*DListNode, 0)
	this.evt_names = make(map[string]IEvent, 0)

	for i := uint64(0); i < this.evt_lay1Size; i++ {
		this.evt_lay1[i].Init(nil)
	}

	for i := 0; i < Tid_last; i++ {
		this.evt_threadMsg[i] = new(DListNode)
		this.evt_threadMsg[i].Init(nil)
	}

	// 日志初始化
	this.log_Buffer = make([]byte, LogBuffMax)
	this.log_BufferLen = 0

	this.log_TimeString = time.Now().Format("15:04:05")
	this.MakeLogHeader()

	if this.is_world_thread() {
		this.log_FileBuff.Grow(LogBuffSize)

		if !IsExist(LogFileName) {
			os.Create(LogFileName)
		}
		file, err := os.OpenFile(LogFileName, os.O_RDWR, os.ModePerm)
		if err != nil {
			return err
		}
		this.log_FileHandle = file
		this.log_FileHandle.Seek(0, 2)
		// 第一条日志
		this.LogDebug("\n          盘古游戏服务器启动\n")
	}

	return nil
}

// 运行线程
func (this *Thread) Run_thread() {
	// 计算心跳误差值, 决定心跳滴答(小数), heart_time, last_time, heart_rate
	// 处理线程间接收消息, 分配到水表定时器
	// 执行水表定时器
	go func() {

		this.start_time = time.Now().UnixNano()
		this.last_time = this.start_time
		next_time := time.Duration(this.heart_time)
		run_time := int64(0)

		this.log_TimeString = time.Now().Format("15:04:05")
		this.MakeLogHeader()

		this.self.on_first_run()

		for {

			time.Sleep(next_time)

			this.log_TimeString = time.Now().Format("15:04:05")
			this.MakeLogHeader()

			this.last_time = time.Now().UnixNano()
			// 设置当前时间戳(毫秒)
			this.get_curr_time_count = 1
			this.curr_time = this.last_time / int64(time.Millisecond)

			// 刷新缓冲日志到文件
			if this.is_world_thread() && this.log_FlushTime < this.curr_time {
				this.log_FlushTime = this.curr_time + ToogoApp.config.LogFlushTime
				if this.log_FileBuff.Len() > 0 {
					this.log_FileHandle.Write(this.log_FileBuff.Bytes())
					this.log_FileBuff.Reset()
				}
			}

			this.self.on_pre_run()

			this.runThreadMsg()
			this.runEvents()
			this.self.on_run()

			this.sendThreadMsg()

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
				this.self.on_end()
				if this.is_world_thread() {
					this.log_FileHandle.Close()
				}
				break
			}
		}

	}()
}

// 返回线程编号
func (this *Thread) Get_thread_id() uint32 {
	return this.id
}

// 返回线程名称
func (this *Thread) Get_thread_name() string {
	return this.name
}

// 是世界线程
func (this *Thread) is_world_thread() bool {
	return this.id == Tid_world
}

// 预备关闭线程
func (this *Thread) pre_close_thread() {
	this.pre_stop = true
}

// 投递定时器事件
func (this *Thread) PostEvent(a IEvent) bool {
	check_name := len(a.GetName()) > 0
	if check_name {
		if _, ok := this.evt_names[a.GetName()]; ok {
			return false
		}
	}

	if a.GetTouchTime() < 0 {
		return false
	}

	// 计算放在那一层
	pos := (a.GetTouchTime() + Evt_gap_time - 1) >> Evt_gap_bit
	if pos < 0 {
		pos = 1
	}

	var header *DListNode

	if pos < this.evt_lay1Size {
		new_pos := this.evt_lay1Cursor + pos
		if new_pos >= this.evt_lay1Size {
			new_pos = new_pos - this.evt_lay1Size
		}
		pos = new_pos
		header = &this.evt_lay1[pos]
	} else {
		if _, ok := this.evt_lay2[pos]; !ok {
			this.evt_lay2[pos] = new(DListNode)
			this.evt_lay2[pos].Init(nil)
		}
		header = this.evt_lay2[pos]
	}

	if header == nil {
		return false
	}

	n := &DListNode{}
	n.Init(a)

	if !a.AddNode(n) {
		return false
	}

	old_pre := header.Pre

	header.Pre = n
	n.Next = header
	n.Pre = old_pre
	old_pre.Next = n

	if check_name {
		this.evt_names[a.GetName()] = a
	}

	return true
}

// 投递线程间消息
func (this *Thread) PostThreadMsg(tid uint32, a IEvent) bool {
	if tid == this.Get_thread_id() {
		this.LogWarn("PostThreadMsg dont post to self")
		return false
	}
	if tid >= Tid_world && tid < Tid_last {
		header := this.evt_threadMsg[tid]

		n := new(DListNode)
		if n == nil {
			this.LogError("PostThreadMsg newDlinkNode failed")
			return false
		}
		n.Init(a)

		if !a.AddNode(n) {
			this.LogError("PostThreadMsg AddNode failed")
			return false
		}

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

// 通过别名获取事件
func (this *Thread) GetEvent(name string) IEvent {
	if _, ok := this.evt_names[name]; ok {
		return this.evt_names[name]
	}
	return nil
}

func (this *Thread) RemoveEvent(e IEvent) {
	delete(this.evt_names, e.GetName())
	e.Destroy()
}

// 接收并处理线程间消息
func (this *Thread) runThreadMsg() {

	header := DListNode{}
	header.Init(nil)

	GetThreadMsgs().GetMsg(this.Get_thread_id(), &header)

	for {
		// 每次得到链表第一个事件(非)
		n := header.Next
		if n.IsEmpty() {
			break
		}

		// 执行事件, 删除这个事件
		e := n.Data.(IEvent)
		e.Exec(this.self)
		e.Destroy()
	}
}

// 发送线程间消息
func (this *Thread) sendThreadMsg() {

	// 发送日志到日志线程
	// if !this.is_world_thread() && this.log_BufferLen > 0 {
	// 	evt := &Event_thread_log{}
	// 	evt.Init("", 100)
	// 	evt.Data = string(this.log_Buffer[:this.log_BufferLen])
	// 	this.PostThreadMsg(Tid_world, evt)

	// 	copy(this.log_Buffer[:0], "")
	// 	this.log_BufferLen = 0
	// }

	for i := uint32(Tid_world); i < Tid_last; i++ {
		if !this.evt_threadMsg[i].IsEmpty() {
			GetThreadMsgs().PostMsg(i, this.evt_threadMsg[i])
		}
	}
}

// 运行一次定时器事件(一个线程心跳可以处理多次)
func (this *Thread) runEvents() {
	all_time := (this.last_time - this.start_time) / int64(time.Millisecond)

	all_count := uint64((all_time + Evt_gap_time - 1) >> Evt_gap_bit)

	for i := this.evt_lastRunCount; i <= all_count; i++ {
		// 执行第一层事件
		this.runExec(&this.evt_lay1[this.evt_lay1Cursor])

		// 执行第二层事件
		if _, ok := this.evt_lay2[this.evt_currRunCount]; ok {
			this.runExec(this.evt_lay2[this.evt_currRunCount])
			delete(this.evt_lay2, this.evt_currRunCount)
		}

		this.evt_currRunCount++
		this.evt_lay1Cursor++
		if this.evt_lay1Cursor >= this.evt_lay1Size {
			this.evt_lay1Cursor = 0
		}
	}

	this.evt_lastRunCount = this.evt_currRunCount
}

// 运行一条定时器事件链表, 每次都执行第一个事件, 直到链表为空
func (this *Thread) runExec(header *DListNode) {
	for {
		// 每次得到链表第一个事件(非)
		n := header.Next
		if n.IsEmpty() {
			break
		}

		d := n.Data.(IEvent)

		// 执行事件, 返回true, 删除这个事件, 返回false表示用户自己处理
		if d.Exec(this.self) {
			this.RemoveEvent(d)
		} else if header.Next == n {
			// 防止使用者没有删除使用过的事件, 造成死循环, 该事件, 用户要么重新投递到其他链表, 要么删除
			this.RemoveEvent(d)
		}
	}
}

// 打印事件池现状
func (this *Thread) PrintAll() {

	fmt.Printf(
		`粒度:%d
		粒度移位:%d
		第一层池容量:%d
		第一层游标:%d
		运行次数%d
		`, Evt_gap_time, Evt_gap_bit, this.evt_lay1Size, this.evt_lay1Cursor, this.evt_currRunCount)

	for k, v := range this.evt_names {
		fmt.Println(k, v)
	}
}

// 线程日志 : 生成日志头
func (this *Thread) MakeLogHeader() {
	id_str := strconv.Itoa(int(this.Get_thread_id()))
	this.log_Header[LogDebugLevel] = this.log_TimeString + " [D] " + id_str + " "
	this.log_Header[LogInfoLevel] = this.log_TimeString + " [I] " + id_str + " "
	this.log_Header[LogWarnLevel] = this.log_TimeString + " [W] " + id_str + " "
	this.log_Header[LogErrorLevel] = this.log_TimeString + " [E] " + id_str + " "
	this.log_Header[LogFatalLevel] = this.log_TimeString + " [F] " + id_str + " "
}

// 线程日志 : 调试[D]级别日志
func (this *Thread) LogDebug(f string, v ...interface{}) {
	this.LogBase(LogDebugLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 信息[I]级别日志
func (this *Thread) LogInfo(f string, v ...interface{}) {
	this.LogBase(LogInfoLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 警告[W]级别日志
func (this *Thread) LogWarn(f string, v ...interface{}) {
	this.LogBase(LogWarnLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 错误[E]级别日志
func (this *Thread) LogError(f string, v ...interface{}) {
	this.LogBase(LogErrorLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 致命[F]级别日志
func (this *Thread) LogFatal(f string, v ...interface{}) {
	this.LogBase(LogFatalLevel, fmt.Sprintf(f, v...))
}

// 线程日志 : 手动分级日志
func (this *Thread) LogBase(level int, info string) {
	if level >= LogDebugLevel && level < LogMaxLevel {
		s := this.log_Header[level] + info
		s = strings.Replace(s, "\n", "\n"+this.log_Header[level], -1) + "\n"

		if this.is_world_thread() {
			this.Add_log(s)
		} else {
			s_len := len(s)
			copy(this.log_Buffer[this.log_BufferLen:], s)
			this.log_BufferLen += s_len
		}

		if level >= LogLimitLevel {
			fmt.Print(s)
		}
	} else {
		fmt.Println("LogBase : level failed : ", level)
	}
}

// 增加日志到缓冲
func (this *Thread) Add_log(d string) {
	if this.is_world_thread() {
		this.log_FileBuff.WriteString(d)
	}
}

// 获取当前时间戳(毫秒)
func (this *Thread) GetCurrTime() int64 {
	this.get_curr_time_count++
	if this.get_curr_time_count > UpdateCurrTimeCount {
		this.get_curr_time_count = 1
		this.curr_time = time.Now().UnixNano() / int64(time.Millisecond)
	}

	return this.curr_time
}
