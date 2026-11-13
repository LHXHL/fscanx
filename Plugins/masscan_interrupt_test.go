package Plugins

import (
	"os"
	"testing"
	"time"
)

func waitInterruptSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func TestWaitMasscanInterruptGraceExpires(t *testing.T) {
	const grace = 50 * time.Millisecond
	sigCh := make(chan os.Signal, 2)
	first := make(chan struct{}, 1)
	forced := make(chan struct{}, 1)
	stop := startMasscanInterrupt(sigCh, grace, func() { first <- struct{}{} }, func() { forced <- struct{}{} })
	defer stop()

	start := time.Now()
	sigCh <- os.Interrupt
	waitInterruptSignal(t, first, "first callback")
	waitInterruptSignal(t, forced, "force callback after grace")

	if elapsed := time.Since(start); elapsed < grace {
		t.Fatalf("force callback ran after %s, want at least %s", elapsed, grace)
	}
	select {
	case <-first:
		t.Fatal("first callback ran twice")
	default:
	}
}

func TestWaitMasscanInterruptSecondSignal(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	first := make(chan struct{}, 1)
	forced := make(chan struct{}, 1)
	stop := startMasscanInterrupt(sigCh, 2*time.Second, func() { first <- struct{}{} }, func() { forced <- struct{}{} })
	defer stop()

	sigCh <- os.Interrupt
	waitInterruptSignal(t, first, "first callback")

	start := time.Now()
	sigCh <- os.Interrupt
	waitInterruptSignal(t, forced, "force callback on second signal")

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("second signal took %s to force quit, want < 1s", elapsed)
	}
	select {
	case <-first:
		t.Fatal("first callback ran twice")
	default:
	}
}

func TestStartMasscanInterruptStopIdempotent(t *testing.T) {
	sigCh := make(chan os.Signal, 1)
	stop := startMasscanInterrupt(sigCh, time.Second, func() {}, func() {})
	stop()
	stop()
}

func TestWaitMasscanInterruptDoneBeforeSignal(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	first := make(chan struct{}, 1)
	forced := make(chan struct{}, 1)
	stop := startMasscanInterrupt(sigCh, time.Second, func() { first <- struct{}{} }, func() { forced <- struct{}{} })
	stop()

	select {
	case <-first:
		t.Fatal("first callback ran after done")
	case <-forced:
		t.Fatal("force callback ran after done")
	default:
	}
}

func TestWaitMasscanInterruptDoneDuringGrace(t *testing.T) {
	sigCh := make(chan os.Signal, 2)
	first := make(chan struct{}, 1)
	forced := make(chan struct{}, 1)
	stop := startMasscanInterrupt(sigCh, time.Second, func() { first <- struct{}{} }, func() { forced <- struct{}{} })

	sigCh <- os.Interrupt
	waitInterruptSignal(t, first, "first callback")
	stop()

	select {
	case <-forced:
		t.Fatal("force callback ran after done during grace")
	default:
	}
}
