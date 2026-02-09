package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const stateDir = "/tmp/alf"

var (
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

func absPath(path string) string {
	out, err := exec.Command("readlink", "-f", path).Output()
	if err != nil {
		return path
	}
	return strings.TrimSpace(string(out))
}

func stopCurrent() {
	mpc("stop")
	mpc("clear")
	os.Remove(fileFile)
}

func isPlaying() bool {
	out, _ := mpc("status")
	return strings.Contains(out, "[playing]")
}

func isPaused() bool {
	out, _ := mpc("status")
	return strings.Contains(out, "[paused]")
}

func play(path string) {
	ensureDir()
	stopCurrent()
	abs := absPath(path)
	os.WriteFile(fileFile, []byte(abs), 0644)
	mpc("clear")
	mpc("add", "file://"+abs)
	mpc("play")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: alf-play <play FILE | stop | pause [FILE] | seek OFFSET | autoplay [on|off|toggle]>")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "play":
		if len(os.Args) >= 3 {
			play(os.Args[2])
		}

	case "stop":
		stopCurrent()

	case "pause":
		if isPlaying() || isPaused() {
			mpc("toggle")
		} else if len(os.Args) >= 3 {
			play(os.Args[2])
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
