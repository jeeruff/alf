package main

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

var audioExt = map[string]bool{
	".wav": true, ".mp3": true, ".flac": true, ".ogg": true,
	".aif": true, ".aiff": true, ".opus": true, ".m4a": true,
	".wma": true, ".ape": true, ".wv": true, ".alac": true,
}

type Meta struct {
	File     string
	BPM      string
	Pitch    string
	Duration string
	Channels string
	Rate     string
	Bits     string
}

func cacheDir() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "alf")
}

func cacheFile(dirpath string) string {
	h := sha256.Sum256([]byte(dirpath))
	return filepath.Join(cacheDir(), fmt.Sprintf("%x.tsv", h[:8]))
}

func readCache(dirpath string) map[string]Meta {
	cache := make(map[string]Meta)
	f, err := os.Open(cacheFile(dirpath))
	if err != nil {
		return cache
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	records, _ := r.ReadAll()
	for _, rec := range records {
		if len(rec) >= 7 {
			cache[rec[0]] = Meta{
				File: rec[0], BPM: rec[1], Pitch: rec[2],
				Duration: rec[3], Channels: rec[4], Rate: rec[5], Bits: rec[6],
			}
		}
	}
	return cache
}

func writeCache(dirpath string, metas []Meta) error {
	os.MkdirAll(cacheDir(), 0755)
	f, err := os.Create(cacheFile(dirpath))
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Comma = '\t'
	for _, m := range metas {
		w.Write([]string{m.File, m.BPM, m.Pitch, m.Duration, m.Channels, m.Rate, m.Bits})
	}
	w.Flush()
	return nil
}

func detectBPM(path string) string {
	out, err := exec.Command("aubiotrack", path).Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return ""
	}
	// average beat interval -> BPM
	var beats []float64
	for _, l := range lines {
		if v, err := strconv.ParseFloat(strings.TrimSpace(l), 64); err == nil {
			beats = append(beats, v)
		}
	}
	if len(beats) < 2 {
		return ""
	}
	totalInterval := beats[len(beats)-1] - beats[0]
	avgInterval := totalInterval / float64(len(beats)-1)
	if avgInterval <= 0 {
		return ""
	}
	bpm := 60.0 / avgInterval
	return fmt.Sprintf("%.0f", bpm)
}

func detectPitch(path string) string {
	out, err := exec.Command("aubiopitch", "-p", "yinfft", path).Output()
	if err != nil {
		return ""
	}
	var sum float64
	var n int
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if v, err := strconv.ParseFloat(fields[1], 64); err == nil && v > 20 {
				sum += v
				n++
			}
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f", sum/float64(n))
}

func getSoxInfo(path string) (dur, ch, rate, bits string) {
	out, err := exec.Command("sox", "--i", path).Output()
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Duration":
			if i := strings.Index(val, " ="); i > 0 {
				dur = val[:i]
			}
		case "Channels":
			ch = val
		case "Sample Rate":
			rate = val
		case "Precision":
			bits = strings.TrimSuffix(val, "-bit")
		}
	}
	return
}

func indexFile(dirpath, name string) Meta {
	path := filepath.Join(dirpath, name)
	dur, ch, rate, bits := getSoxInfo(path)
	bpm := detectBPM(path)
	pitch := detectPitch(path)
	return Meta{
		File: name, BPM: bpm, Pitch: pitch,
		Duration: dur, Channels: ch, Rate: rate, Bits: bits,
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: alf-index <directory> [--force]")
		os.Exit(1)
	}

	dirpath := os.Args[1]
	force := len(os.Args) > 2 && os.Args[2] == "--force"

	// list audio files
	entries, err := os.ReadDir(dirpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "alf-index: %v\n", err)
		os.Exit(1)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && audioExt[strings.ToLower(filepath.Ext(e.Name()))] {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		fmt.Println("no audio files")
		return
	}

	// check existing cache
	existing := readCache(dirpath)
	var toIndex []string
	for _, f := range files {
		if force {
			toIndex = append(toIndex, f)
		} else if _, ok := existing[f]; !ok {
			toIndex = append(toIndex, f)
		}
	}

	if len(toIndex) == 0 {
		fmt.Printf("cache up to date (%d files)\n", len(files))
		return
	}

	fmt.Printf("indexing %d/%d files...\n", len(toIndex), len(files))

	// index in parallel (4 workers)
	results := make(chan Meta, len(toIndex))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)

	for _, name := range toIndex {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fmt.Printf("  %s\n", n)
			results <- indexFile(dirpath, n)
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for m := range results {
		existing[m.File] = m
	}

	// write all back
	var all []Meta
	for _, f := range files {
		if m, ok := existing[f]; ok {
			all = append(all, m)
		}
	}
	if err := writeCache(dirpath, all); err != nil {
		fmt.Fprintf(os.Stderr, "alf-index: write cache: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("done. cached %d files -> %s\n", len(all), cacheFile(dirpath))
}
