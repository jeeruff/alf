package main

import (
	"fmt"
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

// parsePos extracts percent position from mpc status
func parsePos() float64 {
	out, err := mpc("status", "%percenttime%")
	if err != nil {
		return -1
	}
	s := strings.TrimSpace(out)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSpace(s)
	if pct, err := strconv.Atoi(s); err == nil {
		return float64(pct) / 100.0
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

	// refresh loop
	for {
		if !isPlaying() && !isPaused() {
			break
		}
		pos := parsePos()
		if pos >= 0 {
			os.WriteFile(posFile, []byte(fmt.Sprintf("%.4f", pos)), 0644)
			exec.Command("lf", "-remote", fmt.Sprintf("send %s reload", lfID)).Run()
		}
		time.Sleep(300 * time.Millisecond)
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
