package toogo

import (
	"errors"
)

const (
	evt_gap_time  = 16     // 心跳时间(毫秒)
	evt_gap_bit   = 4      // 心跳时间对应得移位(快速运算使用)
	evt_lay1_time = 160000 // 第一层事件池最大支持时间(毫秒)
)

// 事件处理系列接口
type IEventOs interface {
	PostEvent(a IEvent) bool     // 投递定时器事件
	GetEvent(name string) IEvent // 通过别名获取事件
	RemoveEvent(e IEvent)        // 删除事件, 只能操作本线程事件
}

// 线程事件 : 事件系统
type EventOs struct {
	masterSelf       IThread               // 自己, 初始化之后, 不要操作
	evt_lay1         []DListNode           // 第一层事件池
	evt_lay2         map[uint64]*DListNode // 第二层事件池
	evt_names        map[string]IEvent     // 别名
	evt_lay1Size     uint64                // 第一层池容量
	evt_lay1Cursor   uint64                // 第一层游标
	evt_lastRunCount uint64                // 最近一次运行次数
	evt_currRunCount uint64                // 当前运行次数
}

// 线程事件 : 初始化
func (this *EventOs) InitEventOs(masterSelf IThread, lay1_time uint64) error {

	if lay1_time < evt_gap_time || lay1_time > evt_lay1_time {
		return errors.New("[E] 第一层支持16毫秒到160000毫秒")
	}

	if len(this.evt_names) > 0 {
		return errors.New("[E] EventHome 已经初始化过")
	}

	if masterSelf == nil {
		return errors.New("[E] masterSelf 不能为空 ")
	}

	this.masterSelf = masterSelf

	// 初始化事件池
	this.evt_lay1Size = lay1_time >> evt_gap_bit
	this.evt_lay1Cursor = 0
	this.evt_currRunCount = 1
	this.evt_lastRunCount = this.evt_currRunCount

	this.evt_lay1 = make([]DListNode, this.evt_lay1Size)
	this.evt_lay2 = make(map[uint64]*DListNode, 0)
	this.evt_names = make(map[string]IEvent, 0)

	for i := uint64(0); i < this.evt_lay1Size; i++ {
		this.evt_lay1[i].Init(nil)
	}

	return nil
}

// 通过别名获取事件
func (this *EventOs) GetEvent(name string) IEvent {
	if _, ok := this.evt_names[name]; ok {
		return this.evt_names[name]
	}
	return nil
}

func (this *EventOs) RemoveEvent(e IEvent) {
	delete(this.evt_names, e.GetName())
	e.Destroy()
}

// 投递定时器事件
func (this *EventOs) PostEvent(a IEvent) bool {
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
	pos := (a.GetTouchTime() + evt_gap_time - 1) >> evt_gap_bit
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

// 运行一次定时器事件(一个线程心跳可以处理多次)
func (this *EventOs) runEvents(passTime int64) {
	all_count := uint64((passTime + evt_gap_time - 1) >> evt_gap_bit)

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
func (this *EventOs) runExec(header *DListNode) {
	for {
		// 每次得到链表第一个事件(非)
		n := header.Next
		if n.IsEmpty() {
			break
		}

		d := n.Data.(IEvent)

		// 执行事件, 返回true, 删除这个事件, 返回false表示用户自己处理
		if d.Exec(this.masterSelf) {
			this.RemoveEvent(d)
		} else if header.Next == n {
			// 防止使用者没有删除使用过的事件, 造成死循环, 该事件, 用户要么重新投递到其他链表, 要么删除
			this.RemoveEvent(d)
		}
	}
}

// // 打印事件池现状
// func (this *EventOs) PrintAll() {

// 	fmt.Printf(
// 		`粒度:%d
// 		粒度移位:%d
// 		第一层池容量:%d
// 		第一层游标:%d
// 		运行次数%d
// 		`, evt_gap_time, evt_gap_bit, this.evt_lay1Size, this.evt_lay1Cursor, this.evt_currRunCount)

// 	for k, v := range this.evt_names {
// 		fmt.Println(k, v)
// 	}
// }

// 事件 : 线程关闭
type Event_close_thread struct {
	Evt_base
	Master IThread
}

// 事件执行
func (this *Event_close_thread) Exec(home interface{}) bool {
	if this.Master != nil {
		this.Master.Pre_close_thread()
		return true
	}

	LogWarnPost(0, "没找到线程")
	return true
}
