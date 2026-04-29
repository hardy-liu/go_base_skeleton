package command

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestSetupSignalHandlerCancelsOnFirstSignal 验证收到首个退出信号后会触发 cancel，
// 即优雅退出流程被正确开启（而不是直接强制退出）。
func TestSetupSignalHandlerCancelsOnFirstSignal(t *testing.T) {
	// 这里给一个较长超时，确保测试关注点是“首信号触发 cancel”，
	// 而不是被 timeout 分支提前打断。
	cmd, out := startSignalHelper(t, "5s")

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send first signal failed: %v", err)
	}
	// 子进程在 ctx.Done() 后会打印 cancelled，作为 cancel 已触发的可观测信号。
	waitForOutput(t, out, "cancelled", time.Second)

	// 该用例只验证首信号语义，不等待后续强退逻辑，主动回收子进程避免泄漏。
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

// TestSetupSignalHandlerForcesExitOnTimeout 验证首信号后若在超时时间内未完成退出，
// 会走强制退出分支并返回 exit code 1。
func TestSetupSignalHandlerForcesExitOnTimeout(t *testing.T) {
	// 设置较短 timeout，快速触发强制退出路径。
	cmd, out := startSignalHelper(t, "100ms")

	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Fatalf("send first signal failed: %v", err)
	}
	// 先确认 cancel 已发生，再校验后续是否按预期强退。
	waitForOutput(t, out, "cancelled", time.Second)

	err := cmd.Wait()
	if err == nil {
		t.Fatal("expected non-zero exit after shutdown timeout")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
	}
}

// startSignalHelper 启动一个测试子进程，运行 helper 测试入口。
// 子进程内会真实执行 SetupSignalHandler，从而避免在当前测试进程调用 os.Exit。
func startSignalHelper(t *testing.T, timeout string) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()

	// 通过 -test.run 精确命中 helper 入口，避免执行其他测试逻辑。
	cmd := exec.Command(os.Args[0], "-test.run=TestSetupSignalHandlerHelperProcess", "--")
	// 通过环境变量切换到 helper 模式，并传入超时参数。
	cmd.Env = append(os.Environ(),
		"GO_BASE_SKELETON_TEST_SIGNAL_HELPER=1",
		"GO_BASE_SKELETON_TEST_SIGNAL_TIMEOUT="+timeout,
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process failed: %v", err)
	}
	// 等待 helper 完成信号处理器注册，避免主进程过早发信号导致竞态。
	waitForOutput(t, &out, "ready", time.Second)
	return cmd, &out
}

// waitForOutput 轮询等待输出中出现指定 token，用于跨进程同步测试步骤。
func waitForOutput(t *testing.T, out *bytes.Buffer, token string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), token) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for output token %q, current output: %s", token, out.String())
}

// TestSetupSignalHandlerHelperProcess 仅作为子进程入口使用。
// 正常测试流程不会执行其主体代码。
func TestSetupSignalHandlerHelperProcess(t *testing.T) {
	if os.Getenv("GO_BASE_SKELETON_TEST_SIGNAL_HELPER") != "1" {
		return
	}

	timeout, err := time.ParseDuration(os.Getenv("GO_BASE_SKELETON_TEST_SIGNAL_TIMEOUT"))
	if err != nil {
		t.Fatalf("parse helper timeout failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 注册待测信号处理逻辑；主进程会向该子进程发送信号。
	SetupSignalHandler(ctx, cancel, timeout)
	// 输出 ready 表示处理器已就绪，可安全发送信号。
	println("ready")
	// 收到首信号后会 cancel，上报 cancelled 作为断言依据。
	<-ctx.Done()
	println("cancelled")
	// 保持阻塞，让 timeout/第二次信号逻辑有机会触发 os.Exit(1)。
	select {}
}
