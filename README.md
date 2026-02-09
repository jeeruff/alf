# alf — audio lf

lf file manager with ASCII waveform preview for audio files.

```
  amenasty.wav     ▇▃▃▇▇▃▃▇▇▃▃▇▇▄▃▇▇▃▃▇▇▃▃▇█▃▃▇▇▃   10.1s
  noname2128_1.wav ▃█▃▄▂▃▁▂▄▃▄▂▅▄▂▅▆▆▇▇▇▇▇▇▇▇▇▇▇▇   49.7s
  vyberfl_1.wav    ▁▁▂▁█▄▃▄▆▇▆▁▁▁▁▁▁▁▁▁▁▁▁▁▁▂▁▂▅▃  262.2s
```

## what it does

- **preview pane**: full ASCII waveform of selected audio file
- **directory preview**: sparkline list of all audio files in a folder
- **status line**: sparkline + metadata on file select
- everything else: normal lf (all your keybinds, rifle, etc.)

## requires

- [lf](https://github.com/gokcehan/lf)
- [sox](https://sox.sourceforge.net/) (for decoding + soxi metadata)
- python 3

## install

```sh
git clone https://github.com/jeeruff/alf.git
cd alf
sh install.sh
```

## usage

```sh
alf                    # open current dir
alf /path/to/samples   # open sample folder
aw file.wav            # standalone waveform
aw -1 file.wav         # one-line sparkline
aw /path/to/dir        # dir listing with sparklines
```

## how it works

`aw` decodes audio to 8kHz mono via sox, buckets into peak columns,
renders using unicode block characters (▁▂▃▄▅▆▇█). ~50ms per file.

`alf` launches lf with a custom config that sources your main lfrc
and adds waveform preview on top.

## planned

- aubio integration (BPM, pitch, onset detection)
- auto-tagging / metadata columns
- fzf search by audio characteristics
- batch operations (normalize, convert, trim silence)
