#!/usr/bin/env python3
"""aw — ASCII waveform. Modes: full preview, sparkline, directory listing."""
import subprocess, struct, sys, os, argparse

BLOCKS = "▁▂▃▄▅▆▇█"
AUDIO_EXT = {".wav", ".mp3", ".flac", ".ogg", ".aif", ".aiff", ".opus",
             ".m4a", ".wma", ".ape", ".wv", ".alac"}


def decode(path):
    """Decode audio to mono 8kHz s16le via sox."""
    proc = subprocess.run(
        ["sox", path, "-c", "1", "-r", "8000", "-b", "16",
         "-e", "signed-integer", "-t", "raw", "-"],
        capture_output=True, timeout=10
    )
    raw = proc.stdout
    if not raw:
        return []
    return struct.unpack(f"<{len(raw)//2}h", raw)


def peaks_from(samples, width):
    n = len(samples)
    if n == 0:
        return []
    peaks = []
    for i in range(width):
        s, e = i * n // width, (i + 1) * n // width
        chunk = samples[s:e]
        peaks.append(max(abs(x) for x in chunk) if chunk else 0)
    return peaks


def info(path):
    """Audio info via soxi."""
    try:
        parts = {}
        for flag, key in [("-r", "sr"), ("-c", "ch"), ("-b", "bits"), ("-D", "dur")]:
            r = subprocess.run(["soxi", flag, path],
                               capture_output=True, text=True, timeout=2)
            parts[key] = r.stdout.strip()
        dur = float(parts.get("dur", 0))
        return f"{parts.get('bits','')}b {parts.get('sr','')}Hz {parts.get('ch','')}ch", dur
    except:
        return "", 0


def fmt_dur(s):
    if s < 60:
        return f"{s:.1f}s"
    m, s = divmod(s, 60)
    return f"{int(m)}:{s:04.1f}"


def sparkline(path, width=30):
    """Single-line sparkline waveform."""
    samples = decode(path)
    if not samples:
        return "▁" * width, "", 0
    peaks = peaks_from(samples, width)
    mx = max(peaks) or 1
    spark = ""
    for p in peaks:
        lvl = p / mx
        spark += BLOCKS[int(lvl * (len(BLOCKS) - 1))]
    inf, dur = info(path)
    return spark, inf, dur


def full(path, width=80, height=5):
    """Multi-line waveform, top half, baseline at bottom."""
    samples = decode(path)
    if not samples:
        return "  [no audio data]"
    peaks = peaks_from(samples, width)
    mx = max(peaks) or 1

    lines = []
    for row in range(height - 1, -1, -1):
        line = ""
        for p in peaks:
            level = p / mx * height
            if level >= row + 1:
                line += "█"
            elif level > row:
                frac = level - row
                line += BLOCKS[int(frac * (len(BLOCKS) - 1))]
            elif row == 0:
                line += "▁"
            else:
                line += " "
        lines.append(line)

    inf, dur = info(path)
    header = f"  {os.path.basename(path)}  {inf}  [{fmt_dur(dur)}]"
    return header + "\n" + "\n".join(lines)


def dirlist(dirpath, width=80, maxfiles=50):
    """List audio files in directory with sparklines."""
    files = sorted(f for f in os.listdir(dirpath)
                   if os.path.splitext(f)[1].lower() in AUDIO_EXT)
    if not files:
        return "  [no audio files]"

    # calculate column widths
    spark_w = min(30, max(16, width - 40))
    name_w = width - spark_w - 16  # room for dur + info

    out = []
    for f in files[:maxfiles]:
        fpath = os.path.join(dirpath, f)
        try:
            spark, inf, dur = sparkline(fpath, spark_w)
            name = f[:name_w].ljust(name_w)
            out.append(f"  {name} {spark} {fmt_dur(dur):>7}")
        except:
            name = f[:name_w].ljust(name_w)
            out.append(f"  {name} {'·' * spark_w}")

    if len(files) > maxfiles:
        out.append(f"  ... +{len(files) - maxfiles} more")

    return "\n".join(out)


if __name__ == "__main__":
    p = argparse.ArgumentParser(description="aw — ASCII waveform")
    p.add_argument("file", help="audio file or directory")
    p.add_argument("-w", "--width", type=int, default=80)
    p.add_argument("-H", "--height", type=int, default=5)
    p.add_argument("-1", "--oneline", action="store_true", help="sparkline mode")
    p.add_argument("-d", "--dir", action="store_true", help="directory listing mode")
    args = p.parse_args()

    if args.dir or os.path.isdir(args.file):
        print(dirlist(args.file, args.width))
    elif args.oneline:
        spark, inf, dur = sparkline(args.file, args.width)
        print(f"{spark}  {fmt_dur(dur)}  {inf}")
    else:
        print(full(args.file, args.width, args.height))
