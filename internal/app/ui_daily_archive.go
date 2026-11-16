package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"lilt/internal/qqmusic"
)

// What the daily pages say of past picks: the button that leads to the
// days on file, the page that lists them, and the page of one of them.
// See daily_archive.go for the files they live in, and
// daily_archive_load.go for how they load.

// dailyHistoryButton leads to the days on file. It sits with the play
// buttons of the live day and of a kept one, and needs no list loaded
// first: the page itself says what is kept.
func (a *app) dailyHistoryButton(c *ui.Context) {
	if a.profile == nil {
		return
	}
	p := a.pal
	if pillButton(c.Key("history"), nil, "查看历史推荐", p.hover.Over(p.bg), p.text).Clicked() {
		a.open(route{page: pageDailyHistory})
	}
}

// dailyHistoryPage lists the days on file before today as cards, newest
// first; opening one reads what QQ picked that day.
func (a *app) dailyHistoryPage(c *ui.Context) {
	d := a.dailyArchive
	if d == nil || a.profile == nil {
		a.centered(c, "登录 QQ 音乐后，每日推荐会自动留档", nil)
		return
	}
	p := a.pal
	days := d.pastDays(dailyDayNum(time.Now()))
	top, bottom := a.insets(c)
	ui.Scroll(c.Key("daily-history")).TrackScroll(&a.pageState().scroll).Grow(1).
		Padding(top+pageGap, pagePad, bottom+48, pagePad).ScrollbarInsets(top, 0, bottom, 0).
		Children(func() {
			a.startMark(c)
			defer a.endMark(c)
			ui.Column(c).Gap(18).Children(func() {
				ui.Text(c, "历史推荐").FontSize(24).FontWeight(800).LetterSpacing(-0.5)
				switch {
				case d.loading:
					ui.Text(c, "正在读取留档…").FontSize(13).TextColor(p.muted)
				case len(days) == 0:
					ui.Text(c, "还没有更早的留档：应用开着的时候，每天的推荐会自动记下。").
						FontSize(13).TextColor(p.muted)
				default:
					a.dailyHistoryGrid(c, days)
				}
			})
		})
}

// dailyHistoryGrid sets the kept days as the album cards of the purchased
// music are: a grid of covers over the day and its count.
func (a *app) dailyHistoryGrid(c *ui.Context, days []dailyDay) {
	cols := a.columns(c, 156)
	w, _ := c.Size()
	// A card takes what the grid gives it, the gaps between taken out.
	size := (w - sidebarWidth - 2*pagePad - 20*float32(cols-1)) / float32(cols)
	ui.Grid(c).Columns(cols).GapX(20).GapY(28).Children(func() {
		for _, day := range days {
			a.dailyHistoryCard(c, day, size)
		}
	})
}

// dailyHistoryCard is one kept day: its date over its first cover, as the
// day itself was given, over the day and how many songs it kept.
func (a *app) dailyHistoryCard(c *ui.Context, day dailyDay, size float32) {
	p := a.pal
	when := dailyDayTime(day.num)
	label := fmt.Sprintf("%d月%d日 · %s", int(when.Month()), when.Day(), weekday(when))
	card := ui.ButtonBase(c.Key(day.num)).Column().Gap(10).AlignItems(ui.Stretch).FocusRing(false).Label(label)
	hovered := card.Hovered()
	card.Children(func() {
		lift := ui.Pose{}
		cover := ui.Box(c).Size(size, size).Radius(max(8, size*0.06))
		if hovered {
			lift = ui.Pose{Y: -3}
			cover.Shadow(0, 12, 28, -6, p.cardShade)
		} else {
			cover.Shadow(0, 2, 8, -2, p.cardShade)
		}
		cover.AnimatePose(lift, ui.Snappy)
		cover.Children(func() { a.dailyDayArt(c, day.cover, when, size) })
		ui.Text(c, label).FontSize(13).FontWeight(500).MaxLines(2).LineHeight(1.35)
		ui.Text(c, fmt.Sprintf("%d 首", day.count)).FontSize(12).TextColor(p.muted).SingleLine().Margin(-6, 0, 0, 0)
	})
	if card.Clicked() {
		a.open(route{page: pageDaily, id: day.num})
	}
}

// dailyPastPage shows one kept day: the picks QQ gave then, which play
// as they did.
func (a *app) dailyPastPage(c *ui.Context, num int64) {
	d := a.dailyArchive
	if d == nil || a.profile == nil {
		a.centered(c, "登录 QQ 音乐查看每日推荐往期", nil)
		return
	}
	a.loadDailyPast(num)
	kept := d.shown[num]
	if kept != nil && kept.err != "" {
		a.centered(c, kept.err, func() { delete(d.shown, num); a.loadDailyPast(num) })
		return
	}
	var songs []qqmusic.Track
	if kept != nil {
		songs = kept.tracks
	}
	day := dailyDayTime(num)
	s := a.pageShimmer(c)
	header := func(c *ui.Context) {
		p := a.pal
		ui.Row(c).Gap(28).AlignItems(ui.End).Padding(0, 0, 26, 0).Children(func() {
			a.dailyArt(c, songs, day, 212)
			ui.Column(c).Gap(8).Children(func() {
				ui.Text(c, "每日推荐 · 往期").FontSize(12).FontWeight(600).TextColor(p.accent)
				ui.Text(c, fmt.Sprintf("%d月%d日的每日推荐", int(day.Month()), day.Day())).
					FontSize(30).FontWeight(800).LetterSpacing(-0.6)
				if len(songs) == 0 {
					s.text(c, 13, 1.3).Width(240)
					ui.Row(c).Gap(10).Margin(8, 0, 0, 0).Children(func() {
						s.bone(c, 18).Size(112, 36)
						s.bone(c, 18).Size(112, 36)
					})
					return
				}
				ui.Text(c, pastDailyMeta(kept)).FontSize(13).TextColor(p.muted)
				ui.Row(c).Gap(10).Margin(8, 0, 0, 0).Children(func() {
					if pillButton(c.Key("play"), iconPlay, "播放全部", p.accent, p.onAccent).Clicked() {
						if a.pl.Shuffle() {
							a.pl.ToggleShuffle()
						}
						a.pl.PlayQueue(songs, firstPlayable(songs))
					}
					if pillButton(c.Key("shuffle"), iconShuffle, "随机播放", p.hover.Over(p.bg), p.text).Clicked() {
						if !a.pl.Shuffle() {
							a.pl.ToggleShuffle()
						}
						a.pl.PlayQueue(songs, firstPlayable(songs))
					}
					a.dailyHistoryButton(c)
				})
			})
		})
	}
	if len(songs) == 0 {
		a.trackSkeleton(c, s, true, header)
		return
	}
	a.trackRows(c, songs, true, header)
}

// pastDailyMeta says how many songs a kept day holds and when it was read.
func pastDailyMeta(p *dailyPast) string {
	n := len(p.tracks)
	if p.at.IsZero() {
		return fmt.Sprintf("%d 首 · 往期留档", n)
	}
	at := p.at.In(beijing)
	return fmt.Sprintf("%d 首 · 留档于 %d月%d日 %02d:%02d", n, int(at.Month()), at.Day(), at.Hour(), at.Minute())
}
