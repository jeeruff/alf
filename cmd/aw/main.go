package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var blocks = []rune("▁▂▃▄▅▆▇█")

var audioExt = map[string]bool{
	".wav": true, ".mp3": true, ".flac": true, ".ogg": true,
	".aif": true, ".aiff": true, ".opus": true, ".m4a": true,
	".wma": true, ".ape": true, ".wv": true, ".alac": true,
}

const (
	DIM    = "\033[38;5;240m"
	BRIGHT = "\033[0m"
	RST    = "\033[0m"
)

func decode(path string) []int16 {
	cmd := exec.Command("sox", path, "-c", "1", "-r", "8000", "-b", "16",
		"-e", "signed-integer", "-t", "raw", "-")
	raw, err := cmd.Output()
	if err != nil || len(raw) < 2 {
		return nil
	}
	n := len(raw) / 2
	samples := make([]int16, n)
	for i := range n {
		samples[i] = int16(binary.LittleEndian.Uint16(raw[i*2 : i*2+2]))
	}
	return samples
}

func makePeaks(samples []int16, width int) []int16 {
	n := len(samples)
	if n == 0 {
		return nil
	}
	p := make([]int16, width)
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
		p[i] = mx
	}
	return p
}

type audioInfo struct {
	sr, ch, bits string
	dur          float64
}

func getInfo(path string) audioInfo {
	cmd := exec.Command("sox", "--i", path)
	out, err := cmd.Output()
	if err != nil {
		return audioInfo{}
	}
	var info audioInfo
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "Sample Rate":
			info.sr = val
		case "Channels":
			info.ch = val
		case "Precision":
			info.bits = strings.TrimSuffix(val, "-bit")
		case "Duration":
			// parse "00:00:10.07 = 483456 samples..."
			if i := strings.Index(val, " ="); i > 0 {
				val = val[:i]
			}
			fmt.Sscanf(val, "%f", &info.dur)
			// handle HH:MM:SS.ss format
			if strings.Count(val, ":") == 2 {
				var h, m, s float64
				fmt.Sscanf(val, "%f:%f:%f", &h, &m, &s)
				info.dur = h*3600 + m*60 + s
			}
		}
	}
	return info
}

func fmtDur(s float64) string {
	if s < 60 {
		return fmt.Sprintf("%.1fs", s)
	}
	m := int(s) / 60
	sec := s - float64(m*60)
	return fmt.Sprintf("%d:%04.1f", m, sec)
}

type cacheMeta struct {
	BPM, Pitch string
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

func readCacheMeta(path string) cacheMeta {
	dirpath := filepath.Dir(path)
	name := filepath.Base(path)
	f, err := os.Open(cacheFile(dirpath))
	if err != nil {
		return cacheMeta{}
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	records, _ := r.ReadAll()
	for _, rec := range records {
		if len(rec) >= 3 && rec[0] == name {
			return cacheMeta{BPM: rec[1], Pitch: rec[2]}
		}
	}
	return cacheMeta{}
}

func readDirCache(dirpath string) map[string]cacheMeta {
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
		if len(rec) >= 3 {
			cache[rec[0]] = cacheMeta{BPM: rec[1], Pitch: rec[2]}
		}
	}
	return cache
}

func renderFull(path string, width, height int, pos float64) string {
	samples := decode(path)
	if samples == nil {
		return "  [no audio data]"
	}
	peaks := makePeaks(samples, width)
	var mx int16
	for _, p := range peaks {
		if p > mx {
			mx = p
		}
	}
	if mx == 0 {
		mx = 1
	}

	split := -1
	if pos >= 0 {
		split = int(pos * float64(width))
	}

	var sb strings.Builder
	info := getInfo(path)
	cmeta := readCacheMeta(path)
	name := filepath.Base(path)

	// build tag string from cache
	tags := ""
	if cmeta.BPM != "" {
		tags += "  " + cmeta.BPM + "bpm"
	}
	if cmeta.Pitch != "" {
		tags += "  ~" + cmeta.Pitch + "Hz"
	}

	if pos >= 0 {
		cur := info.dur * pos
		sb.WriteString(fmt.Sprintf("  %s  %sb %sHz %sch  [%s / %s]%s\n",
			name, info.bits, info.sr, info.ch, fmtDur(cur), fmtDur(info.dur), tags))
	} else {
		sb.WriteString(fmt.Sprintf("  %s  %sb %sHz %sch  [%s]%s\n",
			name, info.bits, info.sr, info.ch, fmtDur(info.dur), tags))
	}

	for row := height - 1; row >= 0; row-- {
		chars := make([]rune, width)
		for i, p := range peaks {
			level := float64(p) / float64(mx) * float64(height)
			if level >= float64(row+1) {
				chars[i] = '█'
			} else if level > float64(row) {
				frac := level - float64(row)
				idx := int(frac * float64(len(blocks)-1))
				chars[i] = blocks[idx]
			} else if row == 0 {
				chars[i] = '▁'
			} else {
				chars[i] = ' '
			}
		}
		if split >= 0 && split < width {
			sb.WriteString(DIM)
			sb.WriteString(string(chars[:split]))
			sb.WriteString(BRIGHT)
			sb.WriteString(string(chars[split:]))
			sb.WriteString(RST)
		} else if split >= width {
			sb.WriteString(DIM)
			sb.WriteString(string(chars))
			sb.WriteString(RST)
		} else {
			sb.WriteString(string(chars))
		}
		if row > 0 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func renderSparkline(path string, width int) (string, string, float64) {
	samples := decode(path)
	if samples == nil {
		return strings.Repeat("▁", width), "", 0
	}
	peaks := makePeaks(samples, width)
	var mx int16
	for _, p := range peaks {
		if p > mx {
			mx = p
		}
	}
	if mx == 0 {
		mx = 1
	}
	var sb strings.Builder
	for _, p := range peaks {
		lvl := float64(p) / float64(mx)
		idx := int(lvl * float64(len(blocks)-1))
		sb.WriteRune(blocks[idx])
	}
	info := getInfo(path)
	meta := fmt.Sprintf("%sb %sHz %sch", info.bits, info.sr, info.ch)
	return sb.String(), meta, info.dur
}

func renderDir(dirpath string, width, maxfiles int) string {
	entries, err := os.ReadDir(dirpath)
	if err != nil {
		return "  [error reading dir]"
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if audioExt[ext] {
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return "  [no audio files]"
	}
	sort.Strings(files)

	sparkW := width - 40
	if sparkW < 16 {
		sparkW = 16
	}
	if sparkW > 30 {
		sparkW = 30
	}
	nameW := width - sparkW - 16

	dcache := readDirCache(dirpath)

	var sb strings.Builder
	limit := len(files)
	if limit > maxfiles {
		limit = maxfiles
	}
	for i, f := range files[:limit] {
		fpath := filepath.Join(dirpath, f)
		spark, _, dur := renderSparkline(fpath, sparkW)
		name := f
		if len(name) > nameW {
			name = name[:nameW]
		}
		pad := nameW - len(name)
		if pad < 0 {
			pad = 0
		}
		bpm := ""
		if m, ok := dcache[f]; ok && m.BPM != "" {
			bpm = fmt.Sprintf(" %3sbpm", m.BPM)
		}
		sb.WriteString(fmt.Sprintf("  %s%s %s %7s%s", name, strings.Repeat(" ", pad), spark, fmtDur(dur), bpm))
		if i < limit-1 {
			sb.WriteByte('\n')
		}
	}
	if len(files) > maxfiles {
		sb.WriteString(fmt.Sprintf("\n  ... +%d more", len(files)-maxfiles))
	}
	return sb.String()
}

func main() {
	width := flag.Int("w", 80, "width")
	height := flag.Int("H", 5, "height")
	oneline := flag.Bool("1", false, "sparkline mode")
	dir := flag.Bool("d", false, "directory listing")
	pos := flag.Float64("p", -1, "playback position 0.0-1.0")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: aw [flags] <file|dir>")
		os.Exit(1)
	}
	path := flag.Arg(0)

	fi, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "aw: %v\n", err)
		os.Exit(1)
	}

	if *dir || fi.IsDir() {
		fmt.Println(renderDir(path, *width, 50))
	} else if *oneline {
		spark, meta, dur := renderSparkline(path, *width)
		fmt.Printf("%s  %s  %s\n", spark, fmtDur(dur), meta)
	} else {
		fmt.Println(renderFull(path, *width, *height, *pos))
	}
}
