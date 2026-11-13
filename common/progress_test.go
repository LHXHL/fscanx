package common

import (
	"bytes"
	"testing"
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
