package common

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

// 进度行使用 stderr，日志使用 stdout；两者由同一把锁串行化。
// 擦除进度行不用 ANSI 转义，而是 "\r + 等宽空格 + \r"，兼容 Windows 控制台。
var (
	progressOut io.Writer = os.Stderr
	logOut      io.Writer = os.Stdout
	progressTTY           = func() bool {
		fi, err := os.Stderr.Stat()
		return err == nil && fi.Mode()&os.ModeCharDevice != 0
	}

	progressMu      sync.Mutex
	progressEnabled bool
	progressVisible bool
	progressWidth   int
	progressText    string
)

// StartProgress 打开底部进度行。非 TTY 或 -silent 时保持关闭。
func StartProgress() {
	progressMu.Lock()
	defer progressMu.Unlock()
	if Silent || !progressTTY() {
		return
	}
	progressEnabled = true
}

// UpdateProgress 用新文本替换底部进度行。
func UpdateProgress(text string) {
	progressMu.Lock()
	defer progressMu.Unlock()
	if !progressEnabled {
		return
	}
	progressText = text
	if progressVisible {
		clearProgressLocked()
	}
	redrawProgressLocked()
}

// StopProgress 擦除进度行并关闭进度显示。
func StopProgress() {
	progressMu.Lock()
	defer progressMu.Unlock()
	if progressVisible {
		clearProgressLocked()
	}
	progressEnabled = false
	progressText = ""
}

// TryStopProgress 尝试擦除进度行并关闭进度显示；若进度锁已被占用则立即返回。
// 供强制退出路径使用，避免阻塞在可能卡住的输出上。
func TryStopProgress() {
	if !progressMu.TryLock() {
		return
	}
	defer progressMu.Unlock()
	if progressVisible {
		clearProgressLocked()
	}
	progressEnabled = false
	progressText = ""
}

// Println 与 fmt.Println 用法一致，但会先擦掉进度行、打印后再重绘。
func Println(a ...interface{}) {
	withProgress(func() { fmt.Fprintln(logOut, a...) })
}

// Printf 与 fmt.Printf 用法一致，但会先擦掉进度行、打印后再重绘。
func Printf(format string, a ...interface{}) {
	withProgress(func() { fmt.Fprintf(logOut, format, a...) })
}

// withProgress 保证一段输出期间进度行不会和日志交错。
func withProgress(fn func()) {
	progressMu.Lock()
	defer progressMu.Unlock()
	if progressEnabled && progressVisible {
		clearProgressLocked()
	}
	fn()
	if progressEnabled && progressText != "" {
		redrawProgressLocked()
	}
}

func clearProgressLocked() {
	if !progressVisible {
		return
	}
	fmt.Fprint(progressOut, "\r", strings.Repeat(" ", progressWidth), "\r")
	progressVisible = false
}

func redrawProgressLocked() {
	if !progressEnabled || progressText == "" {
		return
	}
	fmt.Fprint(progressOut, progressText)
	progressWidth = utf8.RuneCountInString(progressText)
	progressVisible = true
}
