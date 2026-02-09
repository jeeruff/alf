package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const stateDir = "/tmp/alf"

var (
	mpvSocket    = filepath.Join(stateDir, "mpv")
	pidFile      = filepath.Join(stateDir, "pid")
	posFile      = filepath.Join(stateDir, "pos")
	fileFile     = filepath.Join(stateDir, "file")
	autoplayFile = filepath.Join(stateDir, "autoplay")
)

func ensureDir() {
	os.MkdirAll(stateDir, 0755)
}

func mpvCmd(cmd []any) any {
	conn, err := net.DialTimeout("unix", mpvSocket, 500*time.Millisecond)
	if err != nil {
		return nil
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	msg := map[string]any{"command": cmd}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')
	conn.Write(data)

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil
	}
	var resp map[string]any
	json.Unmarshal(buf[:n], &resp)
	return resp["data"]
}

func stopCurrent() {
	// kill daemon
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			if pid != os.Getpid() {
				syscall.Kill(pid, syscall.SIGTERM)
			}
		}
	}
	// quit mpv
	mpvCmd([]any{"quit"})
	time.Sleep(50 * time.Millisecond)
	// clean state
	for _, f := range []string{posFile, fileFile, pidFile} {
		os.Remove(f)
	}
}

func isPlaying() bool {
	return mpvCmd([]any{"get_property", "pid"}) != nil
}

func getAutoplay() bool {
	_, err := os.Stat(autoplayFile)
	return err == nil
}

func setAutoplay(on bool) {
	ensureDir()
	if on {
		os.WriteFile(autoplayFile, nil, 0644)
	} else {
		os.Remove(autoplayFile)
	}
}

func play(filepath string, lfID string) {
	ensureDir()
	stopCurrent()

	// write PID
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)

	// resolve path
	abs, err := realpath(filepath)
	if err != nil {
		abs = filepath
	}
	os.WriteFile(fileFile, []byte(abs), 0644)

	// remove stale socket
	os.Remove(mpvSocket)

	// start mpv
	cmd := exec.Command("mpv", "--no-terminal", "--no-video",
		"--input-ipc-server="+mpvSocket, abs)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return
	}

	// wait for socket
	for range 30 {
		if _, err := os.Stat(mpvSocket); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// monitor loop
	for {
		if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			break
		}
		// check if process still alive
		if cmd.Process != nil {
			if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
				break
			}
		}

		pct := mpvCmd([]any{"get_property", "percent-pos"})
		if pct != nil {
			if v, ok := pct.(float64); ok {
				os.WriteFile(posFile, []byte(fmt.Sprintf("%.4f", v/100)), 0644)
				exec.Command("lf", "-remote", fmt.Sprintf("send %s reload", lfID)).Run()
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	// wait for process to finish
	cmd.Wait()

	// cleanup
	for _, f := range []string{posFile, fileFile, pidFile} {
		os.Remove(f)
	}
	exec.Command("lf", "-remote", fmt.Sprintf("send %s reload", lfID)).Run()
}

func realpath(path string) (string, error) {
	return os.Readlink(path)
}

func daemonize(fn func()) {
	// double fork
	if os.Getenv("_ALF_DAEMON") == "1" {
		fn()
		return
	}
	os.Setenv("_ALF_DAEMON", "1")
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Start()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: alf-play <play FILE LF_ID | stop | pause [FILE LF_ID] | autoplay [on|off|toggle]>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "play":
		if len(os.Args) < 4 {
			os.Exit(1)
		}
		daemonize(func() {
			play(os.Args[2], os.Args[3])
		})

	case "stop":
		stopCurrent()

	case "pause":
		if isPlaying() {
			mpvCmd([]any{"cycle", "pause"})
		} else if len(os.Args) >= 4 {
			daemonize(func() {
				play(os.Args[2], os.Args[3])
			})
		}

	case "autoplay":
		if len(os.Args) >= 3 {
			switch os.Args[2] {
			case "on":
				setAutoplay(true)
			case "off":
				setAutoplay(false)
			case "toggle":
				setAutoplay(!getAutoplay())
			}
		}
		state := "OFF"
		if getAutoplay() {
			state = "ON"
		}
		fmt.Printf("autoplay: %s\n", state)
	}
}
