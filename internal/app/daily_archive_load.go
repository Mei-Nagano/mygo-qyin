package app

import (
	"slices"
	"time"

	"lilt/internal/qqmusic"
)

// How the day page comes to know the days kept: the list, once per
// account, and the day opened from it, read off the main thread as the
// song shelves are. See daily_archive.go for the files they live in.

// dailyArchiveData is what the day page shows of the days kept: the list
// of them, and the one opened.
type dailyArchiveData struct {
	dir     string     // where this listener's kept days are
	days    []dailyDay // the days kept, newest first
	loading bool       // the days are still listed
	shown   map[int64]*dailyPast
}

// dailyPast is one kept day opened from the list.
type dailyPast struct {
	loading bool
	loaded  bool
	at      time.Time
	tracks  []qqmusic.Track
	err     string
}

// loadDailyArchive lists the days kept for the listener, once per account.
func (a *app) loadDailyArchive() {
	if a.profile == nil || a.dailyArchive != nil {
		return
	}
	d := &dailyArchiveData{dir: dailyArchivePath(a.profile.UserID)}
	a.dailyArchive = d
	if d.dir == "" {
		return
	}
	d.loading = true
	go func() {
		days := dailyDays(d.dir)
		a.post(func() { a.dailyArchiveAnswered(d, days) })
	}()
}

func (a *app) dailyArchiveAnswered(d *dailyArchiveData, days []dailyDay) {
	if a.dailyArchive != d {
		return
	}
	d.loading, d.days = false, days
}

// pastDays is the days kept before today, newest first: today's picks are
// the day page's own, so the list of the past leaves them out.
func (d *dailyArchiveData) pastDays(today int64) []dailyDay {
	past := make([]dailyDay, 0, len(d.days))
	for _, day := range d.days {
		if day.num != today {
			past = append(past, day)
		}
	}
	return past
}

// dailyRecorded folds a freshly kept day into the list of the past.
func (a *app) dailyRecorded(day dailyDay) {
	d := a.dailyArchive
	if d == nil || d.loading || day.num == 0 {
		return
	}
	kept := func(k dailyDay) bool { return k.num == day.num }
	if slices.ContainsFunc(d.days, kept) {
		return
	}
	d.days = append([]dailyDay{day}, d.days...)
}

// loadDailyPast reads one kept day when the listener opens it. After it
// failed, asking again reads it anew.
func (a *app) loadDailyPast(num int64) {
	d := a.dailyArchive
	if d == nil || d.dir == "" {
		return
	}
	if d.shown == nil {
		d.shown = map[int64]*dailyPast{}
	}
	if p := d.shown[num]; p != nil && (p.loading || p.loaded) {
		return
	}
	p := &dailyPast{loading: true}
	d.shown[num] = p
	dir := d.dir
	go func() {
		rec, err := readDaily(dir, num)
		a.post(func() { a.dailyPastAnswered(num, p, rec, err) })
	}()
}

func (a *app) dailyPastAnswered(num int64, p *dailyPast, rec dailyRecord, err error) {
	if a.dailyArchive == nil || a.dailyArchive.shown[num] != p {
		return
	}
	p.loading, p.loaded = false, true
	if err != nil || len(rec.Tracks) == 0 {
		p.tracks = nil
		p.err = "没有找到那天的留档，或它已损坏"
		return
	}
	p.at, p.tracks, p.err = rec.At, rec.Tracks, ""
}

// clearDailyArchive forgets the days kept for the account that is gone,
// as personal snapshots must go with it; see clearSongShelves.
func (a *app) clearDailyArchive() {
	a.dailyArchive = nil
	isPastDay := func(r route) bool { return r.page == pageDaily && r.id != 0 }
	if isPastDay(a.route) {
		a.route = route{page: pageDaily}
	}
	a.history = slices.DeleteFunc(a.history, isPastDay)
	a.forward = slices.DeleteFunc(a.forward, isPastDay)
	for r := range a.pageStates {
		if isPastDay(r) {
			delete(a.pageStates, r)
		}
	}
}
