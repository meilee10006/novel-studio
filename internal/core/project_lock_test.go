package core

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProjectWriteLockBlocksConcurrentMutation(t *testing.T) {
	project, local, _ := newCapabilityPassedProject(t)
	ready := filepath.Join(t.TempDir(), "lock-ready")
	release := filepath.Join(t.TempDir(), "lock-release")
	cmd := exec.Command(os.Args[0], "-test.run=^TestProjectLockHelper$")
	cmd.Env = append(os.Environ(),
		"NOVEL_CORE_LOCK_HELPER=1",
		"NOVEL_CORE_LOCK_ROOT="+local,
		"NOVEL_CORE_LOCK_READY="+ready,
		"NOVEL_CORE_LOCK_RELEASE="+release,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.WriteFile(release, []byte("release"), 0o600)
		_ = cmd.Wait()
	}()
	waitForFile(t, ready)

	err := project.Reconcile()
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "lock") {
		t.Fatalf("Reconcile while another process owns project lock: err=%v", err)
	}
}

func TestProjectLockHelper(t *testing.T) {
	if os.Getenv("NOVEL_CORE_LOCK_HELPER") != "1" {
		return
	}
	root := os.Getenv("NOVEL_CORE_LOCK_ROOT")
	lockPath := filepath.Join(root, "meta", "core", "project.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	if err := os.WriteFile(os.Getenv("NOVEL_CORE_LOCK_READY"), []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(os.Getenv("NOVEL_CORE_LOCK_RELEASE")); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func TestProjectWriteLockCoversMutatingEntryPoints(t *testing.T) {
	project, local, _ := newCapabilityPassedProject(t)
	if err := project.Reconcile(); err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(t.TempDir(), "lock-ready")
	release := filepath.Join(t.TempDir(), "lock-release")
	cmd := exec.Command(os.Args[0], "-test.run=^TestProjectLockHelper$")
	cmd.Env = append(os.Environ(),
		"NOVEL_CORE_LOCK_HELPER=1",
		"NOVEL_CORE_LOCK_ROOT="+local,
		"NOVEL_CORE_LOCK_READY="+ready,
		"NOVEL_CORE_LOCK_RELEASE="+release,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.WriteFile(release, []byte("release"), 0o600)
		_ = cmd.Wait()
	}()
	waitForFile(t, ready)

	checks := map[string]func() error{
		"reconcile": func() error { return project.Reconcile() },
		"scan submission": func() error {
			_, err := project.ScanActiveSubmission()
			return err
		},
		"settle snapshot": func() error {
			_, err := project.SettleActiveSnapshot()
			return err
		},
		"retry submission": func() error {
			_, err := project.RetryInvalidSubmission()
			return err
		},
		"scan control": func() error {
			_, err := project.ScanControlMessage("lock-control")
			return err
		},
		"process control": func() error {
			_, err := project.ProcessControlMessage("lock-control")
			return err
		},
		"settle foundation": func() error {
			_, err := project.SettleFoundation(FoundationSubmission{})
			return err
		},
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); !errors.Is(err, ErrProjectLocked) {
				t.Fatalf("err=%v want ErrProjectLocked", err)
			}
		})
	}
}
