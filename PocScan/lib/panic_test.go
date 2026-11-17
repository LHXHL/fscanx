package lib

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestRunPocTaskPanicRecovery 任务级 panic 恢复（BL-POC-009 执行轮探针实证的原死锁回归锁）：
// runPocTask 内 executePoc(nil,nil) 必触发 nil deref panic——recover 必须捕获且 wg.Done 不丢
// （原实现 recover 在 goroutine 顶层、wg.Done 在循环体内，panic 即 worker 死亡 + Done 跳过
// → CheckMultiPoc 的 wg.Wait 永久挂死）。
func TestRunPocTaskPanicRecovery(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	runPocTask(Task{Req: nil, Poc: nil}, &wg) // p 为 nil：p.Set 解引用即 panic
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("panic 后 wg.Done 丢失（wg.Wait 死锁回归）")
	}
}

// TestCheckMultiPocZeroWorkers 零 worker 守卫：workers<=0 时回退单 worker，tasks<- 不再
// 永久阻塞（零消费者死锁；测试进程未跑 flag.Parse 时 common.PocNum 零值实证）。
func TestCheckMultiPocZeroWorkers(t *testing.T) {
	Inithttp()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	poc := &Poc{Name: "t", Rules: []Rules{{Method: "GET", Path: "/", Expression: "false"}}}
	done := make(chan struct{})
	go func() {
		CheckMultiPoc(req, []*Poc{poc}, 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("workers=0 死锁回归（零消费者阻塞）")
	}
}
