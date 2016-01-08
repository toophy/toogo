package toogo

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// 创建App
func newApp() *App {
	a := new(App)
	a.sessions = make(map[uint64]*Session, 20)
	a.sessionNames = make(map[string]*Session, 16)
	a.sessionTgid = make(map[uint64]uint64, 16)
	a.config.ListenPorts = make(map[string]listenPort, 3)
	a.config.ConnectPorts = make(map[string]connectPort, 9)
	return a
}

// 需求
// 1. 主线程一个, 享有 1 号线程
// 2. 管理网络会话
// 3. 整合框架
type App struct {
	master         IThread             // 主线程
	lastSessionId  uint64              // 网络会话ID
	sessions       map[uint64]*Session // 网络会话池
	sessionNames   map[string]*Session // 网络会话池(别名)
	sessionTgid    map[uint64]uint64   // Tgid对应网络会话
	sessionMutex   sync.RWMutex        // 网络会话池读写锁
	config         toogoConfig         // 配置信息
	wg             sync.WaitGroup      // App退出信号组
	gThreadMsgPool *ThreadMsgPool      // 线程间消息池
}

// 应用初始化
func Run(m IThread) {

	RecoverCommon(0, "App::Run:")

	cfg := &ToogoApp.config
	ToogoApp.gThreadMsgPool = new(ThreadMsgPool)
	ToogoApp.gThreadMsgPool.Init(cfg.MsgPoolCount)

	ToogoApp.master = m
	if ToogoApp.master != nil {
		ToogoApp.master.Run_thread()
	} else {
		fmt.Println("Can not find master thread")
		os.Exit(2)
	}

	for _, v := range cfg.ListenPorts {
		Listen(v.PacketType, Tid_master, v.Name, v.NetType, v.Address, v.AcceptQuit)
	}

	for _, v := range cfg.ConnectPorts {
		Connect(v.PacketType, Tid_master, v.Name, v.NetType, v.Address)
	}

	ToogoApp.wg.Wait()
	<-time.After(6 * time.Second)
	fmt.Println("quit " + ToogoApp.config.AppName)
}

// 进入线程
func EnterThread() {
	ToogoApp.wg.Add(1)
}

// 离开线程
func LeaveThread() {
	ToogoApp.wg.Done()
}

func GetThreadMsgs() *ThreadMsgPool {
	return ToogoApp.gThreadMsgPool
}

// 这个函数, 后期优化, 需要使用 automic 进行等待写入, 主要是强占目标线程会比较厉害, 但是写入操作非常快
func PostThreadMsg(tid uint32, a IThreadMsg) {
	n := new(DListNode)
	n.Init(a)
	GetThreadMsgs().PushOneMsg(tid, n)
}
