package app

import (
	"cmp"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"lilt/internal/qqmusic"
)

// What QQ Music picks for the listener each day is kept as it arrives:
// when the home page loads the day's songs, they are written under
// daily/<uin>/<day>.json in the user data directory, one small file per
// day, so a past day's picks can be read again after QQ has moved on.
// The newest dailyKeep days are kept. Directory and account are resolved
// on the main thread; reads and writes run on goroutines.

// dailyKeep is how many days of past picks the archive holds.
const dailyKeep = 180

// dailyRecord is one kept day, as written to disk.
type dailyRecord struct {
	At     time.Time       `json:"at"`
	Tracks []qqmusic.Track `json:"tracks"`
}

// dailyDayNum is the day whose picks QQ gives at the moment now: QQ
// renews them at 6:00 in Beijing (see dailyRenewed), so the hours before
// six still belong to the day before.
func dailyDayNum(now time.Time) int64 {
	b := now.In(beijing)
	six := time.Date(b.Year(), b.Month(), b.Day(), 6, 0, 0, 0, beijing)
	if b.Before(six) {
		b = b.AddDate(0, 0, -1)
	}
	return int64(b.Year())*10000 + int64(b.Month())*100 + int64(b.Day())
}

// dailyDayTime is midday of a kept day, enough to show and name it.
func dailyDayTime(num int64) time.Time {
	y, m, d := int(num/10000), int(num/100)%100, int(num%100)
	return time.Date(y, time.Month(m), d, 12, 0, 0, 0, beijing)
}

// dailyArchivePath is where one listener's kept days are. It asks the app
// for its directory, so it must run on the main thread; the path it
// answers is safe for goroutines. Empty says there is nothing to keep for.
func dailyArchivePath(uin int64) string {
	if uin <= 0 {
		return ""
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "daily", strconv.FormatInt(uin, 10))
}

// dailyFilePath is where one kept day's songs are.
func dailyFilePath(dir string, num int64) string {
	return filepath.Join(dir, strconv.FormatInt(num, 10)+".json")
}

// dailyDays names the days kept under dir, newest first, with what the
// cards of them show.
func dailyDays(dir string) []dailyDay {
	nums := dailyDayNums(dir)
	days := make([]dailyDay, 0, len(nums))
	for _, num := range nums {
		days = append(days, readDailyDay(dir, num))
	}
	return days
}

// readDailyDay reads one kept day's card: its first cover and its count.
func readDailyDay(dir string, num int64) dailyDay {
	day := dailyDay{num: num}
	b, err := os.ReadFile(dailyFilePath(dir, num))
	if err != nil {
		return day
	}
	var note dailyDayNote
	if json.Unmarshal(b, &note) != nil {
		return day
	}
	day.count = len(note.Tracks)
	if day.count > 0 {
		day.cover = note.Tracks[0].Album.PicURL
	}
	return day
}

// dailyDay is one of the days kept, as the card of it shows it.
type dailyDay struct {
	num   int64
	cover string // the first of its songs' covers
	count int    // how many songs it kept
}

// dailyDayNote is what a kept day's file says of it, read without loading
// its songs: the day's cards need only the first of them.
type dailyDayNote struct {
	Tracks []struct {
		Album struct {
			PicURL string `json:"picUrl"`
		} `json:"al"`
	} `json:"tracks"`
}

// recordDaily keeps one day's picks as they arrive, on a goroutine. A day
// already kept holds what was first read of it. The day written is
// answered, its number 0 when nothing was.
func recordDaily(dir string, songs []qqmusic.Track, now time.Time) dailyDay {
	if dir == "" || len(songs) == 0 {
		return dailyDay{}
	}
	num := dailyDayNum(now)
	path := dailyFilePath(dir, num)
	if _, err := os.Stat(path); err == nil {
		return dailyDay{}
	}
	if os.MkdirAll(dir, 0o700) != nil {
		return dailyDay{}
	}
	b, err := json.Marshal(dailyRecord{At: now, Tracks: songs})
	if err != nil || os.WriteFile(path+".part", b, 0o600) != nil {
		return dailyDay{}
	}
	if os.Rename(path+".part", path) != nil {
		return dailyDay{}
	}
	pruneDaily(dir)
	return dailyDay{num: num, cover: songs[0].Album.PicURL, count: len(songs)}
}

// pruneDaily drops all but the newest dailyKeep days.
func pruneDaily(dir string) {
	nums := dailyDayNums(dir)
	if len(nums) <= dailyKeep {
		return
	}
	for _, num := range nums[dailyKeep:] {
		os.Remove(dailyFilePath(dir, num))
	}
}

// dailyDayNums names the days kept under dir, newest first.
func dailyDayNums(dir string) []int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var nums []int64
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		num, err := strconv.ParseInt(strings.TrimSuffix(name, ".json"), 10, 64)
		if err == nil && num > 0 {
			nums = append(nums, num)
		}
	}
	slices.SortFunc(nums, func(a, b int64) int { return cmp.Compare(b, a) })
	return nums
}

// readDaily loads one kept day's songs.
func readDaily(dir string, num int64) (dailyRecord, error) {
	b, err := os.ReadFile(dailyFilePath(dir, num))
	if err != nil {
		return dailyRecord{}, err
	}
	var rec dailyRecord
	err = json.Unmarshal(b, &rec)
	return rec, err
}
