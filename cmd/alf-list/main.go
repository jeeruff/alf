package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

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

var blocks = []rune("▁▂▃▄▅▆▇█")

var audioExt = map[string]bool{
	".wav": true, ".mp3": true, ".flac": true, ".ogg": true,
	".aif": true, ".aiff": true, ".opus": true, ".m4a": true,
	".wma": true, ".ape": true, ".wv": true, ".alac": true,
}

type entry struct {
	name  string
	spark string
	bpm   int
	key   string
	pitch float64
	dur   float64
	size  int64
	info  string // "24b 48000Hz 2ch"
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

type cacheMeta struct {
	BPM, Pitch, Dur, Ch, Rate, Bits, Spark string
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
			m := cacheMeta{
				BPM: rec[1], Pitch: rec[2], Dur: rec[3],
				Ch: rec[4], Rate: rec[5], Bits: rec[6],
			}
			if len(rec) >= 8 {
				m.Spark = rec[7]
			}
			cache[rec[0]] = m
		}
	}
	return cache
}

func miniSparkline(path string, width int) string {
	cmd := exec.Command("sox", path, "-c", "1", "-r", "8000", "-b", "16",
		"-e", "signed-integer", "-t", "raw", "-")
	raw, err := cmd.Output()
	if err != nil || len(raw) < 2 {
		return strings.Repeat(string(blocks[0]), width)
	}
	n := len(raw) / 2
	samples := make([]int16, n)
	for i := range n {
		samples[i] = int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
	}
	peaks := make([]int16, width)
	for i := range width {
		s := i * n / width
		e := (i + 1) * n / width
		var mx int16
		for _, v := range samples[s:e] {
			if v < 0 {
				v = -v
			}
			if v > mx {
				mx = v
			}
		}
		peaks[i] = mx
	}
	var maxP int16
	for _, p := range peaks {
		if p > maxP {
			maxP = p
		}
	}
	if maxP == 0 {
		maxP = 1
	}
	var sb strings.Builder
	for _, p := range peaks {
		lvl := float64(p) / float64(maxP)
		sb.WriteRune(blocks[int(lvl*float64(len(blocks)-1))])
	}
	return sb.String()
}

func fmtDur(s float64) string {
	if s < 60 {
		return fmt.Sprintf("%.1fs", s)
	}
	m := int(s) / 60
	sec := s - float64(m*60)
	return fmt.Sprintf("%d:%04.1f", m, sec)
}

func fmtSize(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.0fK", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func parseDur(s string) float64 {
	// "00:01:40.59" or "00:00:10.07"
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ":")
	if len(parts) == 3 {
		h, _ := strconv.ParseFloat(parts[0], 64)
		m, _ := strconv.ParseFloat(parts[1], 64)
		sec, _ := strconv.ParseFloat(parts[2], 64)
		return h*3600 + m*60 + sec
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func main() {
	sortBy := flag.String("sort", "name", "sort by: name, bpm, key, dur, size")
	sparkW := flag.Int("spark", 20, "sparkline width")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: alf-list [--sort name|bpm|dur|size] [--spark N] <directory>")
		os.Exit(1)
	}
	dirpath := flag.Arg(0)

	abs, err := filepath.Abs(dirpath)
	if err != nil {
		abs = dirpath
	}

	entries_raw, err := os.ReadDir(abs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "alf-list: %v\n", err)
		os.Exit(1)
	}

	cache := readCache(abs)

	var entries []entry
	for _, e := range entries_raw {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !audioExt[ext] {
			continue
		}
		fpath := filepath.Join(abs, e.Name())
		fi, _ := e.Info()
		var sz int64
		if fi != nil {
			sz = fi.Size()
		}

		var bpm int
		var dur float64
		var key string
		var pitch float64
		var info string
		var spark string
		if m, ok := cache[e.Name()]; ok {
			bpm, _ = strconv.Atoi(m.BPM)
			dur = parseDur(m.Dur)
			key = hzToNote(m.Pitch)
			pitch, _ = strconv.ParseFloat(m.Pitch, 64)
			info = fmt.Sprintf("%sb %sHz %sch", m.Bits, m.Rate, m.Ch)
			if m.Spark != "" && len([]rune(m.Spark)) == *sparkW {
				spark = m.Spark
			}
		}
		if spark == "" {
			spark = miniSparkline(fpath, *sparkW)
		}

		entries = append(entries, entry{
			name:  e.Name(),
			spark: spark,
			bpm:   bpm,
			key:   key,
			pitch: pitch,
			dur:   dur,
			size:  sz,
			info:  info,
		})
	}

	// sort
	switch *sortBy {
	case "bpm":
		sort.Slice(entries, func(i, j int) bool { return entries[i].bpm < entries[j].bpm })
	case "key":
		sort.Slice(entries, func(i, j int) bool { return entries[i].pitch < entries[j].pitch })
	case "dur":
		sort.Slice(entries, func(i, j int) bool { return entries[i].dur < entries[j].dur })
	case "size":
		sort.Slice(entries, func(i, j int) bool { return entries[i].size < entries[j].size })
	default:
		sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	}

	// output: sparkline | bpm | key | dur | size | name
	for _, e := range entries {
		bpmStr := "   "
		if e.bpm > 0 {
			bpmStr = fmt.Sprintf("%3d", e.bpm)
		}
		keyStr := fmt.Sprintf("%-3s", e.key)
		durStr := fmt.Sprintf("%7s", fmtDur(e.dur))
		sizeStr := fmt.Sprintf("%5s", fmtSize(e.size))
		fmt.Printf("%s  %s  %s  %s  %s  %s\n", e.spark, bpmStr, keyStr, durStr, sizeStr, e.name)
	}
}
