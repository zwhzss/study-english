//go:build windows

package ai

import (
	"os/exec"
	"syscall"
)

// hideWindow 隐藏子进程控制台窗口（Windows）：避免每次 edge-tts 合成弹出黑窗
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
