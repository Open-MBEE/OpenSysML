package subprocess

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

func sh(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh on PATH")
	}
	return path
}

func TestOutputReturnsBothStreams(t *testing.T) {
	stdout, stderr, err := Output(time.Minute, 1<<10, sh(t), "-c", "echo out; echo err >&2")
	if err != nil || string(stdout) != "out\n" || string(stderr) != "err\n" {
		t.Fatalf("got %q, %q, %v", stdout, stderr, err)
	}
}

func TestOutputKeepsTheExitError(t *testing.T) {
	_, _, err := Output(time.Minute, 1<<10, sh(t), "-c", "exit 3")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Fatalf("got %v, want exit status 3", err)
	}
}

func TestOutputStopsAProgramThatNeverExits(t *testing.T) {
	start := time.Now()
	_, _, err := Output(200*time.Millisecond, 1<<10, sh(t), "-c", "exec sleep 600")
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("got %v, want ErrTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > time.Minute {
		t.Fatalf("returned after %s", elapsed)
	}
}

func TestOutputStopsAProgramThatFloodsAStream(t *testing.T) {
	for _, stream := range []string{"", " >&2"} {
		stdout, stderr, err := Output(time.Minute, 1<<10, sh(t), "-c", "while :; do echo flooding"+stream+"; done")
		if !errors.Is(err, ErrOutput) {
			t.Fatalf("stream %q: got %v, want ErrOutput", stream, err)
		}
		if len(stdout) > 1<<10 || len(stderr) > 1<<10 {
			t.Fatalf("stream %q: kept %d and %d bytes past the limit", stream, len(stdout), len(stderr))
		}
	}
}

func TestTimeoutOfZeroIsNoLimit(t *testing.T) {
	ctx, cancel := Timeout(context.Background(), 0)
	defer cancel()
	if _, bounded := ctx.Deadline(); bounded || ctx.Err() != nil {
		t.Fatalf("a zero timeout gave a deadline or an expired context: %v", ctx.Err())
	}
	ctx, cancel = Timeout(context.Background(), time.Hour)
	defer cancel()
	if _, bounded := ctx.Deadline(); !bounded {
		t.Fatal("a positive timeout gave no deadline")
	}
}
