package common

import (
	"bytes"
	"testing"
	"time"
)

func TestProgressCoordinatedOutput(t *testing.T) {
	oldSilent := Silent
	oldOut := progressOut
	oldLog := logOut
	oldTTY := progressTTY
	defer func() {
		StopProgress()
		Silent = oldSilent
		progressOut = oldOut
		logOut = oldLog
		progressTTY = oldTTY
	}()

	Silent = false
	progressTTY = func() bool { return true }
	var out bytes.Buffer
	progressOut = &out
	logOut = &out
	StopProgress()

	StartProgress()
	UpdateProgress("P")
	Println("L")
	StopProgress()

	want := "P\r \rL\nP\r \r"
	if out.String() != want {
		t.Fatalf("progress/log interleave = %q, want %q", out.String(), want)
	}
}

func TestTryStopProgressDoesNotBlock(t *testing.T) {
	oldSilent := Silent
	oldOut := progressOut
	oldTTY := progressTTY
	defer func() {
		StopProgress()
		Silent = oldSilent
		progressOut = oldOut
		progressTTY = oldTTY
	}()

	Silent = false
	progressTTY = func() bool { return true }
	var out bytes.Buffer
	progressOut = &out
	StopProgress()
	StartProgress()
	UpdateProgress("P")

	progressMu.Lock()
	done := make(chan struct{})
	go func() {
		TryStopProgress()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		progressMu.Unlock()
		t.Fatal("TryStopProgress blocked while progressMu was held")
	}
	progressMu.Unlock()

	if !progressEnabled || !progressVisible {
		t.Fatal("TryStopProgress changed state while progressMu was held")
	}
	before := out.Len()
	TryStopProgress()
	if progressEnabled || progressVisible {
		t.Fatal("TryStopProgress did not clear progress state")
	}
	if out.Len() <= before {
		t.Fatal("TryStopProgress did not write clear sequence")
	}
}

func TestProgressDisabledWhenNotTTY(t *testing.T) {
	oldSilent := Silent
	oldOut := progressOut
	oldLog := logOut
	oldTTY := progressTTY
	defer func() {
		StopProgress()
		Silent = oldSilent
		progressOut = oldOut
		logOut = oldLog
		progressTTY = oldTTY
	}()

	Silent = false
	progressTTY = func() bool { return false }
	var out bytes.Buffer
	progressOut = &out
	logOut = &out
	StopProgress()

	StartProgress()
	UpdateProgress("P")
	Println("L")
	StopProgress()

	if out.String() != "L\n" {
		t.Fatalf("non-TTY output = %q, want %q", out.String(), "L\n")
	}
}
