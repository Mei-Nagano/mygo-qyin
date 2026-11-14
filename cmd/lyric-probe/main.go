// Command lyric-probe reports what the lyric endpoints return, so that
// missing or partial lyrics can be traced to a stage: the request, the QRC
// decode, or its parsing into lines.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"lilt/internal/lyrics"
	"lilt/internal/qqmusic"
)

var qrcHeader = regexp.MustCompile(`(?m)^\s*\[\d+,\d+\]`)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "charts" {
		charts()
		return
	}
	queries := []string{"周杰伦", "Taylor Swift", "米津玄師", "IU", "纯音乐", "林俊杰"}
	if len(os.Args) > 1 {
		queries = os.Args[1:]
	}
	for _, q := range queries {
		res, err := qqmusic.SearchPage(q, qqmusic.SearchSongs, 6, 0)
		if err != nil {
			fmt.Printf("search %q: %v\n", q, err)
			continue
		}
		fmt.Printf("\n=== %s (%d) ===\n", q, len(res.Songs))
		for _, tr := range res.Songs {
			report(tr)
		}
	}
}

// report prints what came back for one track.
func report(tr qqmusic.Track) {
	qqmusic.RememberTracks([]qqmusic.Track{tr})
	lrc, trans, qrc, err := qqmusic.Lyrics(tr.ID)
	if err != nil {
		fmt.Printf("  ERR   %-28.28s %-14.14s %v\n", tr.Name, tr.ArtistNames(), err)
		return
	}
	lines := lyrics.Build(lrc, trans, qrc)
	spread := false
	if len(lines) == 0 && !lyrics.IsInstrumental(lrc) {
		if lines = lyrics.Spread(lrc, tr.Length(), tr.Name); len(lines) > 0 {
			spread = true
		}
	}
	headers := len(qrcHeader.FindAllString(qrc, -1))
	last := ""
	if n := len(lines); n > 0 {
		last = lines[n-1].Text
		if len([]rune(last)) > 12 {
			last = string([]rune(last)[:12])
		}
	}
	note := ""
	switch {
	case spread:
		note = "  (untimed, spread)"
	case lines == nil && lyrics.IsInstrumental(lrc):
		note = "  (instrumental)"
	case lines == nil:
		note = "  <-- NOTHING"
	case headers > len(lines):
		note = fmt.Sprintf("  <-- QRC %d lines, %d kept", headers, len(lines))
	}
	fmt.Printf("  %-28.28s %-14.14s qrc=%d lrc=%d trans=%d lines=%d last=%q%s\n",
		tr.Name, tr.ArtistNames(), len(qrc), len(lrc), len(trans), len(lines), strings.TrimSpace(last), note)
}

// charts sweeps the charts' tracks, the way the app plays them, and counts
// what came back short.
func charts() {
	lists, err := qqmusic.Charts()
	if err != nil {
		fmt.Println("charts:", err)
		return
	}
	bad, total, instrumental := 0, 0, 0
	for _, pl := range lists {
		detail, err := qqmusic.PlaylistDetail(pl.ID)
		if err != nil || detail == nil {
			continue
		}
		fmt.Printf("\n=== %s (%d tracks) ===\n", pl.Name, len(detail.Tracks))
		for _, tr := range detail.Tracks {
			total++
			qqmusic.RememberTracks([]qqmusic.Track{tr})
			lrc, trans, qrc, err := qqmusic.Lyrics(tr.ID)
			if err != nil {
				bad++
				fmt.Printf("  ERR  %-26.26s %-14.14s %v\n", tr.Name, tr.ArtistNames(), err)
				continue
			}
			lines := lyrics.Build(lrc, trans, qrc)
			headers := len(qrcHeader.FindAllString(qrc, -1))
			switch {
			case lines == nil && lyrics.IsInstrumental(lrc):
				instrumental++
			case lines == nil:
				bad++
				fmt.Printf("  NONE %-26.26s %-14.14s lrc=%d qrc=%d trans=%d\n", tr.Name, tr.ArtistNames(), len(lrc), len(qrc), len(trans))
			case headers > len(lines):
				bad++
				fmt.Printf("  SHORT %-25.25s %-14.14s qrc %d lines, %d kept\n", tr.Name, tr.ArtistNames(), headers, len(lines))
			}
		}
	}
	fmt.Printf("\ntotal=%d problems=%d instrumental=%d\n", total, bad, instrumental)
}
