package toogo

import (
	"sync"
)

// Go程间消息存放处
type ThreadMsgPool struct {
	lock      []sync.Mutex      // silk消息池有一个独立的锁
	cond      []sync.Cond       // 条件锁
	header    []DListNode       // silk消息池
	count     uint32            // 以上3个数组的容量
	free_lock sync.Mutex        // 邮箱空闲列表锁
	frees     map[uint32]uint32 // 邮箱空闲列表
}

// 初始化
func (this *ThreadMsgPool) Init(count uint32) {
	this.count = count
	this.lock = make([]sync.Mutex, this.count)
	this.cond = make([]sync.Cond, this.count)
	this.header = make([]DListNode, this.count)

	for i := uint32(0); i < this.count; i++ {
		this.cond[i].L = &this.lock[i]
		this.header[i].Init(nil)
	}

	this.frees = make(map[uint32]uint32, this.count)
	for i := uint32(0); i < this.count; i++ {
		this.frees[i] = i
	}
}

// 获取一个
func (this *ThreadMsgPool) AllocId() (uint32, bool) {
	this.free_lock.Lock()
	defer this.free_lock.Unlock()

	if len(this.frees) > 0 {
		for k, _ := range this.frees {
			delete(this.frees, k)
			return k, true
		}
	}

	return 0, false
}

// 释放一个
func (this *ThreadMsgPool) FreeId(id uint32) {
	this.free_lock.Lock()
	defer this.free_lock.Unlock()

	if id < this.count {
		this.frees[id] = id
	}
}

// 投递Go程间消息, PostMsg和GetMsg是一对
func (this *ThreadMsgPool) PostMsg(tid uint32, e *DListNode) bool {
	if e != nil && !e.IsEmpty() && tid < this.count {
		this.lock[tid].Lock()
		defer this.lock[tid].Unlock()

		header := &this.header[tid]

		e_pre := e.Pre
		e_next := e.Next

		e.Init(nil)

		old_pre := header.Pre

		header.Pre = e_pre
		e_pre.Next = header

		e_next.Pre = old_pre
		old_pre.Next = e_next

		return true
	}
	return false
}

// 获取Go程间消息, PostMsg和GetMsg是一对
func (this *ThreadMsgPool) GetMsg(tid uint32, e *DListNode) bool {
	if e != nil && tid < this.count {
		this.lock[tid].Lock()
		defer this.lock[tid].Unlock()

		header := &this.header[tid]

		if !header.IsEmpty() {
			header_pre := header.Pre
			header_next := header.Next

			header.Init(nil)

			old_pre := e.Pre

			e.Pre = header_pre
			header_pre.Next = e

			header_next.Pre = old_pre
			old_pre.Next = header_next
		}

		return true
	}
	return false
}

// 投递Go程间消息, PushMsg 和 WaitMsg 是一对, 投递和等待消息
func (this *ThreadMsgPool) PushOneMsg(tid uint32, e *DListNode) bool {

	if e != nil && tid < this.count {
		this.lock[tid].Lock()
		defer this.lock[tid].Unlock()

		header := &this.header[tid]

		old_pre := header.Pre

		header.Pre = e
		e.Next = header

		e.Pre = old_pre
		old_pre.Next = e

		this.cond[tid].Signal()

		return true
	}
	return false
}

// 投递Go程间消息, PushMsg 和 WaitMsg 是一对, 投递和等待消息
func (this *ThreadMsgPool) PushMsg(tid uint32, e *DListNode) bool {

	if e != nil && !e.IsEmpty() && tid < this.count {
		this.lock[tid].Lock()
		defer this.lock[tid].Unlock()

		header := &this.header[tid]

		e_pre := e.Pre
		e_next := e.Next

		e.Init(nil)

		old_pre := header.Pre

		header.Pre = e_pre
		e_pre.Next = header

		e_next.Pre = old_pre
		old_pre.Next = e_next

		this.cond[tid].Signal()

		return true
	}
	return false
}

// 等待Go程间消息, PushMsg 和 WaitMsg 是一对, 投递和等待消息
func (this *ThreadMsgPool) WaitMsg(tid uint32, e *DListNode) bool {
	if e != nil && tid >= 0 && tid < this.count {
		this.cond[tid].L.Lock()
		defer this.cond[tid].L.Unlock()

		this.cond[tid].Wait()

		header := &this.header[tid]

		if !header.IsEmpty() {
			header_pre := header.Pre
			header_next := header.Next

			header.Init(nil)

			old_pre := e.Pre

			e.Pre = header_pre
			header_pre.Next = e

			header_next.Pre = old_pre
			old_pre.Next = header_next
		}

		return true
	}
	return false
}

var g_ThreadMsgPool *ThreadMsgPool

// 单例
func GetThreadMsgs() *ThreadMsgPool {
	if g_ThreadMsgPool == nil {
		g_ThreadMsgPool = new(ThreadMsgPool)
	}

	return g_ThreadMsgPool
}
