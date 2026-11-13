package Plugins

import (
	"os"
	"sync"
	"time"
)

// masscanInterruptGrace 是第一次 Ctrl+C 后等待在飞扫描收尾的上限。
const masscanInterruptGrace = 3 * time.Second

// waitMasscanInterrupt 实现 -msc 的两段式中断策略：
// 第一次信号调用 onFirst；随后在 grace 内等待第二次信号或 done 关闭。
// 第二次信号或计时到期调用 onForce；done 关闭表示正常完成，直接返回。
func waitMasscanInterrupt(sigCh <-chan os.Signal, done <-chan struct{}, grace time.Duration, onFirst, onForce func()) {
	select {
	case <-sigCh:
	case <-done:
		return
	}

	onFirst()

	timer := time.NewTimer(grace)
	defer timer.Stop()

	select {
	case <-sigCh:
	case <-timer.C:
	case <-done:
		return
	}
	onForce()
}

// startMasscanInterrupt 在后台运行 waitMasscanInterrupt，返回停止函数。
// 停止函数关闭 done 并等待 handler 退出；若 onForce 执行 os.Exit，则不会返回。
func startMasscanInterrupt(sigCh <-chan os.Signal, grace time.Duration, onFirst, onForce func()) func() {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		waitMasscanInterrupt(sigCh, done, grace, onFirst, onForce)
	}()
	var stopOnce sync.Once
	return func() {
		stopOnce.Do(func() {
			close(done)
			<-finished
		})
	}
}
