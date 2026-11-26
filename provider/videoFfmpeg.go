package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type FFmpegStatus struct {
	Installed        bool   `json:"installed"`
	Ready            bool   `json:"ready"`
	Path             string `json:"path"`
	FFprobeInstalled bool   `json:"ffprobe_installed"`
	FFprobePath      string `json:"ffprobe_path"`
	Version          string `json:"version"`
	Platform         string `json:"platform"`
	CanInstall       bool   `json:"can_install"`
	Message          string `json:"message"`
}

type FFmpegInstallEvent struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
	Command string `json:"command,omitempty"`
}

type ffmpegInstallDeps struct {
	goos     string
	geteuid  func() int
	lookPath func(string) (string, error)
	run      func(context.Context, string, []string, []string, func(string)) error
}

func defaultFFmpegInstallDeps() ffmpegInstallDeps {
	return ffmpegInstallDeps{
		goos:     runtime.GOOS,
		geteuid:  os.Geteuid,
		lookPath: exec.LookPath,
		run:      runFFmpegCommand,
	}
}

func (d ffmpegInstallDeps) withDefaults() ffmpegInstallDeps {
	defaults := defaultFFmpegInstallDeps()
	if d.goos == "" {
		d.goos = defaults.goos
	}
	if d.geteuid == nil {
		d.geteuid = defaults.geteuid
	}
	if d.lookPath == nil {
		d.lookPath = defaults.lookPath
	}
	if d.run == nil {
		d.run = defaults.run
	}
	return d
}

var (
	ffmpegPathMu    sync.Mutex
	ffmpegPath      string
	ffprobePathMu   sync.Mutex
	ffprobePath     string
	ffmpegInstallMu sync.Mutex
)

func DetectFFmpeg() FFmpegStatus {
	return detectFFmpegWithDeps(defaultFFmpegInstallDeps())
}

func detectFFmpegWithDeps(deps ffmpegInstallDeps) FFmpegStatus {
	deps = deps.withDefaults()
	status := FFmpegStatus{
		Platform:   deps.goos,
		CanInstall: deps.goos == "linux",
	}
	path, version, err := detectFFmpegBinary(deps, "ffmpeg")
	if err != nil {
		status.Message = err.Error()
	} else {
		status.Installed = true
		status.Path = path
		status.Version = version
	}
	probePath, _, probeErr := detectFFmpegBinary(deps, "ffprobe")
	if probeErr == nil {
		status.FFprobeInstalled = true
		status.FFprobePath = probePath
	}
	status.Ready = status.Installed && status.FFprobeInstalled
	if status.Ready {
		status.Message = "ffmpeg and ffprobe are ready"
	} else if status.Message != "" && probeErr != nil {
		status.Message += "; " + probeErr.Error()
	} else if probeErr != nil {
		status.Message = probeErr.Error()
	}
	return status
}

func detectFFmpegBinary(deps ffmpegInstallDeps, name string) (string, string, error) {
	path, err := deps.lookPath(name)
	if err != nil {
		return "", "", fmt.Errorf("%s not found: %w", name, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	lines := make([]string, 0, 2)
	if err := deps.run(ctx, path, []string{"-version"}, nil, func(line string) {
		if len(lines) < 2 {
			lines = append(lines, line)
		}
	}); err != nil {
		return "", "", fmt.Errorf("%s is not usable: %w", name, err)
	}
	version := ""
	if len(lines) > 0 {
		version = lines[0]
	}
	return path, version, nil
}

func getFfmpegPath() (string, error) {
	ffmpegPathMu.Lock()
	defer ffmpegPathMu.Unlock()
	if ffmpegPath != "" {
		return ffmpegPath, nil
	}
	status := DetectFFmpeg()
	if !status.Installed {
		return "", errors.New(status.Message)
	}
	ffmpegPath = status.Path
	return ffmpegPath, nil
}

func getFFprobePath() (string, error) {
	ffprobePathMu.Lock()
	defer ffprobePathMu.Unlock()
	if ffprobePath != "" {
		return ffprobePath, nil
	}
	status := DetectFFmpeg()
	if !status.FFprobeInstalled {
		return "", errors.New(status.Message)
	}
	ffprobePath = status.FFprobePath
	return ffprobePath, nil
}

func refreshFFmpegPaths() {
	ffmpegPathMu.Lock()
	ffmpegPath = ""
	ffmpegPathMu.Unlock()
	ffprobePathMu.Lock()
	ffprobePath = ""
	ffprobePathMu.Unlock()
}

func InstallFFmpeg(ctx context.Context, emit func(FFmpegInstallEvent)) error {
	if !ffmpegInstallMu.TryLock() {
		return errors.New("another ffmpeg installation is already running")
	}
	defer ffmpegInstallMu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	return installFFmpegWithDeps(ctx, defaultFFmpegInstallDeps(), emit)
}

func installFFmpegWithDeps(ctx context.Context, deps ffmpegInstallDeps, emit func(FFmpegInstallEvent)) error {
	deps = deps.withDefaults()
	if emit == nil {
		emit = func(FFmpegInstallEvent) {}
	}
	emit(FFmpegInstallEvent{Stage: "detect", Message: "正在检测 ffmpeg 与 ffprobe"})
	if deps.goos != "linux" {
		return fmt.Errorf("automatic ffmpeg installation is only supported on Linux servers")
	}
	status := detectFFmpegWithDeps(deps)
	if status.Ready {
		emit(FFmpegInstallEvent{Stage: "done", Message: "ffmpeg 与 ffprobe 已安装"})
		return nil
	}

	prefix := []string{}
	if deps.geteuid() != 0 {
		emit(FFmpegInstallEvent{Stage: "permission", Message: "当前用户不是 root，正在检查免密 sudo"})
		if err := deps.run(ctx, "sudo", []string{"-n", "true"}, nil, func(line string) {
			emit(FFmpegInstallEvent{Stage: "output", Output: line})
		}); err != nil {
			return fmt.Errorf("ffmpeg installation requires root or passwordless sudo: %w", err)
		}
		prefix = append(prefix, "sudo")
	}

	manager, err := findPackageManager(deps.lookPath)
	if err != nil {
		return err
	}
	emit(FFmpegInstallEvent{Stage: "package_manager", Message: "使用包管理器 " + manager})
	commands := packageManagerCommands(manager)
	for _, command := range commands {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := manager
		args := append([]string{}, command.args...)
		if len(prefix) > 0 {
			name = "sudo"
			args = append([]string{"-n", manager}, args...)
		}
		emit(FFmpegInstallEvent{
			Stage:   command.stage,
			Message: command.message,
			Command: strings.Join(append([]string{name}, args...), " "),
		})
		if err := deps.run(ctx, name, args, command.env, func(line string) {
			emit(FFmpegInstallEvent{Stage: "output", Output: line})
		}); err != nil {
			return fmt.Errorf("%s failed: %w", command.stage, err)
		}
	}

	refreshFFmpegPaths()
	emit(FFmpegInstallEvent{Stage: "verify", Message: "正在验证安装结果"})
	status = detectFFmpegWithDeps(deps)
	if !status.Ready {
		return fmt.Errorf("ffmpeg installation completed but binaries are not ready: %s", status.Message)
	}
	emit(FFmpegInstallEvent{Stage: "done", Message: "ffmpeg 与 ffprobe 安装完成"})
	return nil
}

type packageManagerCommand struct {
	stage   string
	message string
	args    []string
	env     []string
}

func findPackageManager(lookPath func(string) (string, error)) (string, error) {
	for _, manager := range []string{"apt-get", "dnf", "yum", "apk", "pacman", "zypper"} {
		path, err := lookPath(manager)
		if err == nil && path != "" {
			return path, nil
		}
	}
	return "", errors.New("no supported Linux package manager found (apt-get/dnf/yum/apk/pacman/zypper)")
}

func packageManagerCommands(manager string) []packageManagerCommand {
	switch filepath.Base(strings.TrimSpace(manager)) {
	case "apt-get":
		return []packageManagerCommand{
			{stage: "update", message: "正在更新软件包索引", args: []string{"update"}},
			{
				stage:   "install",
				message: "正在安装 ffmpeg",
				args:    []string{"install", "-y", "ffmpeg"},
				env:     []string{"DEBIAN_FRONTEND=noninteractive"},
			},
		}
	case "dnf":
		return []packageManagerCommand{{stage: "install", message: "正在安装 ffmpeg", args: []string{"install", "-y", "ffmpeg"}}}
	case "yum":
		return []packageManagerCommand{{stage: "install", message: "正在安装 ffmpeg", args: []string{"install", "-y", "ffmpeg"}}}
	case "apk":
		return []packageManagerCommand{{stage: "install", message: "正在安装 ffmpeg", args: []string{"add", "--no-cache", "ffmpeg"}}}
	case "pacman":
		return []packageManagerCommand{{stage: "install", message: "正在安装 ffmpeg", args: []string{"-Sy", "--noconfirm", "ffmpeg"}}}
	case "zypper":
		return []packageManagerCommand{{stage: "install", message: "正在安装 ffmpeg", args: []string{"--non-interactive", "install", "ffmpeg"}}}
	default:
		return []packageManagerCommand{{stage: "install", message: "正在安装 ffmpeg", args: []string{"install", "-y", "ffmpeg"}}}
	}
}

func runFFmpegCommand(
	ctx context.Context,
	name string,
	args []string,
	env []string,
	emit func(string),
) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	writer := &ffmpegOutputWriter{emit: emit}
	cmd.Stdout = writer
	cmd.Stderr = writer
	err := cmd.Run()
	writer.flush()
	return err
}

type ffmpegOutputWriter struct {
	emit    func(string)
	buffer  []byte
	discard bool
}

func (w *ffmpegOutputWriter) Write(data []byte) (int, error) {
	written := len(data)
	for len(data) > 0 {
		newline := -1
		for i, value := range data {
			if value == '\n' {
				newline = i
				break
			}
		}
		part := data
		if newline >= 0 {
			part = data[:newline]
		}
		if !w.discard {
			remaining := 64*1024 - len(w.buffer)
			if len(part) > remaining {
				part = part[:remaining]
				w.discard = true
			}
			w.buffer = append(w.buffer, part...)
		}
		if newline < 0 {
			break
		}
		w.flush()
		data = data[newline+1:]
	}
	return written, nil
}

func (w *ffmpegOutputWriter) flush() {
	if len(w.buffer) == 0 {
		w.discard = false
		return
	}
	line := strings.TrimSpace(string(w.buffer))
	if len(line) > 4096 {
		line = line[:4096]
	}
	if line != "" && w.emit != nil {
		w.emit(line)
	}
	w.buffer = w.buffer[:0]
	w.discard = false
}
