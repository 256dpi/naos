package tree

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func execPTY(t *testing.T, script string) (string, error) {
	t.Helper()

	// prepare a build tree to run in
	base := t.TempDir()
	err := os.Mkdir(Directory(base), 0755)
	if err != nil {
		t.Fatal(err)
	}

	// run script in a pty
	var buf bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- Exec(base, &buf, strings.NewReader(""), true, true, "sh", "-c", script)
	}()

	// await result
	select {
	case err := <-done:
		return buf.String(), err
	case <-time.After(10 * time.Second):
		t.Fatal("timeout")
		return "", nil
	}
}

func TestExecPTYSuccess(t *testing.T) {
	out, err := execPTY(t, "echo hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected output, got %q", out)
	}
}

func TestExecPTYFailure(t *testing.T) {
	_, err := execPTY(t, "exit 3")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "3") {
		t.Fatalf("expected exit status 3, got %v", err)
	}
}

func TestExecPTYDrainsOutput(t *testing.T) {
	out, _ := execPTY(t, "for i in $(seq 1 2000); do echo line-$i; done")
	if !strings.Contains(out, "line-2000") {
		t.Fatalf("expected last line, got %d bytes ending in %q", len(out), out[max(0, len(out)-40):])
	}
}
