package toogo

// 事件 : 线程关闭
type Event_close_thread struct {
	Evt_base
	Master IThread
}

// 事件执行
func (this *Event_close_thread) Exec(home interface{}) bool {
	if this.Master != nil {
		this.Master.pre_close_thread()
		return true
	}

	println("没找到线程")
	return true
}
