package lyrics

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// QQ's lyrics hold double quotes of their own, left unescaped inside the
	// attribute: the content runs to the quote that ends the attribute, not
	// to the first one, which would cut the song short.
	qrcContent = regexp.MustCompile(`\bLyricContent="`)
	qrcAttrEnd = regexp.MustCompile(`"\s*(?:/?>|[A-Za-z_][\w:.-]*\s*=)`)
	qrcWord    = regexp.MustCompile(`\((\d+),(\d+)\)`)
)

// ParseQRC reads QQ's word-timed lyrics, either their XML wrapper or its
// LyricContent. Each text fragment precedes its (absolute start, duration)
// in milliseconds; literal parentheses and timed spaces remain in the text.
func ParseQRC(qrc string) []Line {
	if m := qrcContent.FindStringIndex(qrc); m != nil {
		// Extract the attribute before unescaping it: escaped quotes belong
		// to the lyrics, and the real newlines separate their timed lines.
		content := qrc[m[1]:]
		if e := qrcAttrEnd.FindStringIndex(content); e != nil {
			content = content[:e[0]]
		}
		qrc = html.UnescapeString(content)
	}
	ms := func(s string) time.Duration {
		n, _ := strconv.ParseInt(s, 10, 64)
		return time.Duration(n) * time.Millisecond
	}
	var lines []Line
	for _, raw := range strings.Split(qrc, "\n") {
		raw = strings.TrimSpace(raw)
		header := yrcLine.FindStringSubmatch(raw)
		if header == nil {
			continue
		}
		line := Line{At: ms(header[1]), Timed: true}
		line.End = line.At + ms(header[2])
		body := raw[len(header[0]):]
		at := 0
		for _, m := range qrcWord.FindAllStringSubmatchIndex(body, -1) {
			text := body[at:m[0]]
			at = m[1]
			if text == "" {
				continue // An empty timed unit carries no text to highlight.
			}
			start := ms(body[m[2]:m[3]])
			word := Word{Text: text, Start: start, End: start + ms(body[m[4]:m[5]])}
			line.Words = append(line.Words, word)
			line.Text += text
			line.End = max(line.End, word.End)
		}
		line.Text = strings.TrimSpace(line.Text)
		// Text the timings do not cover still belongs to the line: dropping
		// the line would lose the words altogether.
		if tail := strings.TrimSpace(body[at:]); tail != "" {
			line.Text = strings.TrimSpace(line.Text + tail)
		}
		if line.Text != "" {
			lines = append(lines, line)
		}
	}
	sortLines(lines)
	return lines
}
