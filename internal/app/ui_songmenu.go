package app

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"lilt/internal/qqmusic"
)

// copyText puts text on the clipboard; tests replace it.
var copyText = func(text string) error { return mygo.Clipboard.Write(transfer.TextData(text)) }

// songMenu is three dots that open the menu of tr.
func (a *app) songMenu(c *ui.Context, tr *qqmusic.Track, size float32, color ui.Color, surface ...buttonSurface) ui.Element {
	return a.songMenuItems(c, a.songItems(tr, 0), size, color, surface...)
}

// nowPlayingMenu is the dots of the page playing tr: its menu, and the level
// of the streams asked for, changed as it plays.
func (a *app) nowPlayingMenu(c *ui.Context, tr *qqmusic.Track, size float32, color ui.Color, surface ...buttonSurface) ui.Element {
	return a.songMenuItems(c, func(m *ui.Menu) {
		a.songItems(tr, 0)(m)
		m.Separator()
		a.qualityItems(m)
	}, size, color, surface...)
}

// songMenuItems is the dots that open the menu items fills.
func (a *app) songMenuItems(c *ui.Context, items func(m *ui.Menu), size float32, color ui.Color, surface ...buttonSurface) ui.Element {
	b := iconButton(c, iconMore, size, color, "更多", surface...)
	b.Menu(items)
	return b
}

// qualityItems lets the listener change the level of the streams from beside
// what plays: the next song asks for that one, as from the settings.
func (a *app) qualityItems(m *ui.Menu) {
	it := m.Submenu("音质", func(sub *ui.Menu) {
		for _, q := range qualityLevels {
			chosen := q.level == a.settings.Quality
			if sub.Item(q.label).Radio(chosen).Chosen() {
				a.settings.Quality = q.level
				a.settings.save()
				a.toast = "音质：" + qualityName(q.level)
				a.redraw()
			}
		}
	})
	it.Trailing(qualityName(a.settings.Quality))
}

// qualityName is a level of streams named shortly, without its detail.
func qualityName(level string) string {
	for _, q := range qualityLevels {
		if q.level == level {
			if i := strings.Index(q.label, "（"); i > 0 {
				return q.label[:i]
			}
			return q.label
		}
	}
	return qualityLevels[1].label
}

// songItems builds the menu of what can be done with tr, behind its dots
// or a right click on its row: playing it next, copying its title, its title
// with its artists, or the link to its page, opening its artists' or album's, and,
// signed in, adding it to a playlist or taking it out of pid, the playlist
// it is listed in, if set.
func (a *app) songItems(tr *qqmusic.Track, pid int64) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if tr.ProgramID != 0 {
			a.programItems(m, tr)
			return
		}
		if m.Item("下一首播放").Disabled(!tr.Playable()).Chosen() {
			a.pl.PlayNext(*tr)
			a.toast = "已设为下一首播放"
		}
		m.Separator()
		if m.Item("复制歌名").Chosen() {
			copyText(tr.Name)
		}
		if m.Item("复制歌名和歌手").Chosen() {
			copyText(tr.Name + " - " + tr.ArtistNames())
		}
		m.Separator()
		if m.Item("复制链接").Chosen() {
			copyText(tr.URL())
		}
		// Its artists' and album's pages, those NetEase has.
		linked := tr.Album.ID != 0
		for _, ar := range tr.Artists {
			linked = linked || ar.ID != 0
		}
		if linked {
			m.Separator()
		}
		for _, ar := range tr.Artists {
			if ar.ID != 0 && m.Item("查看歌手："+ar.Name).Chosen() {
				a.openArtist(ar.ID)
			}
		}
		if tr.Album.ID != 0 && m.Item("查看专辑："+tr.Album.Name).Chosen() {
			a.openPage(route{page: pageAlbum, id: tr.Album.ID})
		}
		a.playlistItems(m, tr, pid)
	}
}

// programItems builds the menu of an episode: copying its title or the
// link to its page, or opening its podcast's.
func (a *app) programItems(m *ui.Menu, tr *qqmusic.Track) {
	if m.Item("复制节目名").Chosen() {
		copyText(tr.Name)
	}
	if m.Item("复制链接").Chosen() {
		copyText(tr.URL())
	}
	m.Separator()
	if m.Item("查看播客").Chosen() {
		a.openPage(route{page: pagePodcast, id: tr.PodcastID})
	}
}
