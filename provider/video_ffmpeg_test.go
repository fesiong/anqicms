package provider

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

type ffmpegTestCommand struct {
	name string
	args []string
	env  []string
}

func TestDetectFFmpegReportsUsableBinaries(t *testing.T) {
	deps := ffmpegInstallDeps{
		goos:    "linux",
		geteuid: func() int { return 0 },
		lookPath: func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		},
		run: func(_ context.Context, name string, args []string, _ []string, emit func(string)) error {
			if args[0] == "-version" {
				emit(name + " version 7.1")
			}
			return nil
		},
	}
	status := detectFFmpegWithDeps(deps)
	if !status.Installed || !status.Ready || !status.CanInstall {
		t.Fatalf("status = %#v", status)
	}
	if status.Path != "/usr/bin/ffmpeg" || status.FFprobePath != "/usr/bin/ffprobe" {
		t.Fatalf("paths = %#v", status)
	}
	if !strings.Contains(status.Version, "7.1") {
		t.Fatalf("version = %q", status.Version)
	}
}

func TestDetectFFmpegBoundsVersionProbe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		release := make(chan struct{})
		deps := ffmpegInstallDeps{
			lookPath: func(name string) (string, error) {
				return "/usr/bin/" + name, nil
			},
			run: func(ctx context.Context, _ string, _ []string, _ []string, _ func(string)) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-release:
					return errors.New("released")
				}
			},
		}
		done := make(chan error, 1)
		go func() {
			_, _, err := detectFFmpegBinary(deps, "ffmpeg")
			done <- err
		}()
		synctest.Wait()
		time.Sleep(11 * time.Second)
		synctest.Wait()
		var probeErr error
		completed := false
		select {
		case probeErr = <-done:
			completed = true
		default:
			close(release)
			<-done
		}
		if !completed {
			t.Fatal("ffmpeg version probe did not respect its timeout")
		}
		if !errors.Is(probeErr, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want probe deadline", probeErr)
		}
	})
}

func TestInstallFFmpegUsesFixedAPTArguments(t *testing.T) {
	commands := make([]ffmpegTestCommand, 0)
	events := make([]FFmpegInstallEvent, 0)
	installed := false
	deps := ffmpegInstallDeps{
		goos:    "linux",
		geteuid: func() int { return 0 },
		lookPath: func(name string) (string, error) {
			if name == "ffmpeg" || name == "ffprobe" {
				if installed {
					return "/usr/bin/" + name, nil
				}
				return "", errors.New("not installed")
			}
			if name == "apt-get" {
				return "/usr/bin/apt-get", nil
			}
			return "", errors.New("not found")
		},
		run: func(_ context.Context, name string, args []string, env []string, emit func(string)) error {
			commands = append(commands, ffmpegTestCommand{name: name, args: append([]string(nil), args...), env: append([]string(nil), env...)})
			emit("installing ffmpeg")
			if len(commands) == 2 {
				installed = true
			}
			return nil
		},
	}
	if err := installFFmpegWithDeps(context.Background(), deps, func(event FFmpegInstallEvent) {
		events = append(events, event)
	}); err != nil {
		t.Fatal(err)
	}
	want := []ffmpegTestCommand{
		{name: "/usr/bin/apt-get", args: []string{"update"}},
		{name: "/usr/bin/apt-get", args: []string{"install", "-y", "ffmpeg"}, env: []string{"DEBIAN_FRONTEND=noninteractive"}},
		{name: "/usr/bin/ffmpeg", args: []string{"-version"}},
		{name: "/usr/bin/ffprobe", args: []string{"-version"}},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	if len(events) == 0 || events[len(events)-1].Stage != "done" {
		t.Fatalf("events = %#v", events)
	}
	foundOutput := false
	for _, event := range events {
		if event.Stage == "output" && event.Output == "installing ffmpeg" {
			foundOutput = true
		}
	}
	if !foundOutput {
		t.Fatalf("missing real command output: %#v", events)
	}
}

func TestInstallFFmpegRunsPackageManagerThroughSudo(t *testing.T) {
	commands := make([]ffmpegTestCommand, 0)
	installed := false
	deps := ffmpegInstallDeps{
		goos:    "linux",
		geteuid: func() int { return 1000 },
		lookPath: func(name string) (string, error) {
			if name == "ffmpeg" || name == "ffprobe" {
				if installed {
					return "/usr/bin/" + name, nil
				}
				return "", errors.New("not installed")
			}
			if name == "apt-get" {
				return "/usr/bin/apt-get", nil
			}
			return "", errors.New("not found")
		},
		run: func(_ context.Context, name string, args []string, env []string, _ func(string)) error {
			commands = append(commands, ffmpegTestCommand{name: name, args: append([]string(nil), args...), env: append([]string(nil), env...)})
			if len(args) > 0 && args[len(args)-1] == "ffmpeg" {
				installed = true
			}
			return nil
		},
	}
	if err := installFFmpegWithDeps(context.Background(), deps, func(FFmpegInstallEvent) {}); err != nil {
		t.Fatal(err)
	}
	want := []ffmpegTestCommand{
		{name: "sudo", args: []string{"-n", "true"}},
		{name: "sudo", args: []string{"-n", "/usr/bin/apt-get", "update"}},
		{name: "sudo", args: []string{"-n", "/usr/bin/apt-get", "install", "-y", "ffmpeg"}, env: []string{"DEBIAN_FRONTEND=noninteractive"}},
		{name: "/usr/bin/ffmpeg", args: []string{"-version"}},
		{name: "/usr/bin/ffprobe", args: []string{"-version"}},
	}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestInstallFFmpegRequiresPasswordlessSudo(t *testing.T) {
	commands := make([]ffmpegTestCommand, 0)
	deps := ffmpegInstallDeps{
		goos:    "linux",
		geteuid: func() int { return 1000 },
		lookPath: func(name string) (string, error) {
			if name == "apt-get" {
				return "/usr/bin/apt-get", nil
			}
			return "", errors.New("not installed")
		},
		run: func(_ context.Context, name string, args []string, _ []string, _ func(string)) error {
			commands = append(commands, ffmpegTestCommand{name: name, args: append([]string(nil), args...)})
			return errors.New("sudo: a password is required")
		},
	}
	err := installFFmpegWithDeps(context.Background(), deps, func(FFmpegInstallEvent) {})
	if err == nil || !strings.Contains(err.Error(), "root") {
		t.Fatalf("error = %v", err)
	}
	want := []ffmpegTestCommand{{name: "sudo", args: []string{"-n", "true"}}}
	if !reflect.DeepEqual(commands, want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
}

func TestInstallFFmpegRejectsNonLinux(t *testing.T) {
	deps := ffmpegInstallDeps{goos: "darwin", geteuid: func() int { return 0 }}
	err := installFFmpegWithDeps(context.Background(), deps, func(FFmpegInstallEvent) {})
	if err == nil || !strings.Contains(err.Error(), "Linux") {
		t.Fatalf("error = %v", err)
	}
}
