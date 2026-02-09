package main

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var audioExt = map[string]bool{
	".wav": true, ".mp3": true, ".flac": true, ".ogg": true,
	".aif": true, ".aiff": true, ".opus": true, ".m4a": true,
	".wma": true, ".ape": true, ".wv": true, ".alac": true,
}

var noteNames = []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}

func hzToNote(hzStr string) string {
	hz, err := strconv.ParseFloat(hzStr, 64)
	if err != nil || hz <= 20 {
		return ""
	}
	midi := 69.0 + 12.0*math.Log2(hz/440.0)
	note := int(math.Round(midi))
	if note < 0 || note > 127 {
		return ""
	}
	return fmt.Sprintf("%s%d", noteNames[note%12], note/12-1)
}

type cacheMeta struct {
	BPM, Pitch, Spark string
}

func cacheFile(dirpath string) string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	h := sha256.Sum256([]byte(dirpath))
	return filepath.Join(dir, "alf", fmt.Sprintf("%x.tsv", h[:8]))
}

func readCache(dirpath string) map[string]cacheMeta {
	cache := make(map[string]cacheMeta)
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
			m := cacheMeta{BPM: rec[1], Pitch: rec[2]}
			if len(rec) >= 8 {
				m.Spark = rec[7]
			}
			cache[rec[0]] = m
		}
	}
	return cache
}

func escLf(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}

func main() {
	if len(os.Args) < 2 {
		return
	}

	// determine directory from first file arg
	dirpath := filepath.Dir(os.Args[1])
	abs, err := filepath.Abs(dirpath)
	if err != nil {
		abs = dirpath
	}

	cache := readCache(abs)
	if len(cache) == 0 {
		return
	}

	var cmds []string
	for _, arg := range os.Args[1:] {
		name := filepath.Base(arg)
		ext := strings.ToLower(filepath.Ext(name))
		if !audioExt[ext] {
			continue
		}

		m, ok := cache[name]
		if !ok {
			continue
		}

		// build info string: spark bpm key
		var parts []string
		if m.Spark != "" {
			parts = append(parts, m.Spark)
		}
		if m.BPM != "" {
			parts = append(parts, fmt.Sprintf("%3s", m.BPM))
		} else {
			parts = append(parts, "   ")
		}
		if note := hzToNote(m.Pitch); note != "" {
			parts = append(parts, fmt.Sprintf("%-3s", note))
		}

		info := strings.Join(parts, " ")
		cmds = append(cmds, fmt.Sprintf(`addcustominfo "%s" "%s"`, escLf(arg), escLf(info)))
	}

	if len(cmds) > 0 {
		fmt.Print(strings.Join(cmds, "; "))
	}
}
