//go:build linux

package protocol

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadUTF8RejectsFIFOWithoutBlocking(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "input.json")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := ReadUTF8(root, "input.json", 1024)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("FIFO unexpectedly accepted as protocol text")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("ReadUTF8 blocked while opening FIFO")
	}
}
