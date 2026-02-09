package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const stateDir = "/tmp/alf"

var (
	pidFile      = filepath.Join(stateDir, "pid")
	posFile      = filepath.Join(stateDir, "pos")
	fileFile     = filepath.Join(stateDir, "file")
	autoplayFile = filepath.Join(stateDir, "autoplay")
)

func mpdHost() string {
	h := os.Getenv("MPD_HOST")
	if h != "" {
		return h
	}
	home, _ := os.UserHomeDir()
	sock := filepath.Join(home, ".config/mpd/socket")
	if _, err := os.Stat(sock); err == nil {
		return sock
	}
	return "127.0.0.1"
}

func mpc(args ...string) (string, error) {
	cmd := exec.Command("mpc", args...)
	cmd.Env = append(os.Environ(), "MPD_HOST="+mpdHost())
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func ensureDir() { os.MkdirAll(stateDir, 0755) }

func stopCurrent() {
	// kill refresh daemon
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, _ := strconv.Atoi(strings.TrimSpace(string(data))); pid > 0 && pid != os.Getpid() {
			proc, _ := os.FindProcess(pid)
			if proc != nil {
				proc.Signal(os.Interrupt)
			}
		}
	}
	mpc("stop")
	mpc("clear")
	for _, f := range []string{posFile, fileFile, pidFile} {
		os.Remove(f)
	}
}

func isPlaying() bool {
	out, _ := mpc("status")
	return strings.Contains(out, "[playing]")
}

func isPaused() bool {
	out, _ := mpc("status")
	return strings.Contains(out, "[paused]")
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

func parseTime(s string) float64 {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 2 {
		m, _ := strconv.ParseFloat(parts[0], 64)
		sec, _ := strconv.ParseFloat(parts[1], 64)
		return m*60 + sec
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parsePos extracts fractional position from mpc status elapsed/total time
func parsePos() float64 {
	out, err := mpc("status")
	if err != nil {
		return -1
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "[") {
			continue
		}
		// "[playing] #1/1   0:05/0:30 (16%)"
		for _, field := range strings.Fields(line) {
			if strings.Count(field, "/") == 1 && strings.Contains(field, ":") {
				times := strings.SplitN(field, "/", 2)
				elapsed := parseTime(times[0])
				total := parseTime(times[1])
				if total > 0 {
					return elapsed / total
				}
			}
		}
	}
	return -1
}

func play(filepath string, lfID string) {
	ensureDir()
	stopCurrent()

	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)

	abs, err := absPath(filepath)
	if err != nil {
		abs = filepath
	}
	os.WriteFile(fileFile, []byte(abs), 0644)

	mpc("clear")
	mpc("add", "file://"+abs)
	mpc("play")

	// refresh loop — only reload when position changes visibly
	var lastPos float64 = -1
	for {
		if !isPlaying() && !isPaused() {
			break
		}
		pos := parsePos()
		if pos >= 0 && (lastPos < 0 || math.Abs(pos-lastPos) >= 0.005) {
			os.WriteFile(posFile, []byte(fmt.Sprintf("%.4f", pos)), 0644)
			exec.Command("lf", "-remote", fmt.Sprintf("send %s reload", lfID)).Run()
			lastPos = pos
		}
		time.Sleep(500 * time.Millisecond)
	}

	// cleanup
	for _, f := range []string{posFile, fileFile, pidFile} {
		os.Remove(f)
	}
	exec.Command("lf", "-remote", fmt.Sprintf("send %s reload", lfID)).Run()
}

func absPath(path string) (string, error) {
	out, err := exec.Command("readlink", "-f", path).Output()
	if err != nil {
		return path, err
	}
	return strings.TrimSpace(string(out)), nil
}

func daemonize(fn func()) {
	if os.Getenv("_ALF_DAEMON") == "1" {
		fn()
		return
	}
	env := append(os.Environ(), "_ALF_DAEMON=1")
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Env = env
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
		daemonize(func() { play(os.Args[2], os.Args[3]) })

	case "stop":
		stopCurrent()

	case "pause":
		if isPlaying() || isPaused() {
			mpc("toggle")
		} else if len(os.Args) >= 4 {
			daemonize(func() { play(os.Args[2], os.Args[3]) })
		}

	case "seek":
		if len(os.Args) >= 3 {
			mpc("seek", os.Args[2])
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
