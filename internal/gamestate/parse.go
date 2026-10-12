package gamestate

import (
	"regexp"
	"strconv"
	"strings"
)

type Kind int

const (
	KindState Kind = iota + 1
	KindScoreSaved
	KindReplaySaved
	KindExit
)

type Line struct {
	Kind       Kind
	State      string
	ScoreID    int64
	ReplayPath string
}

var (
	reState  = regexp.MustCompile("Screen has been switched to type: `(\\w+)`")
	reScore  = regexp.MustCompile(`Successfully saved score to database with id: (\d+)`)
	reReplay = regexp.MustCompile(`Successfully saved replay to path: (.+)$`)
	reExit   = regexp.MustCompile(`Discord RPC has shutdown`)
)

// ParseLine extracts the events we care about from one runtime.log line.
func ParseLine(s string) (Line, bool) {
	if m := reState.FindStringSubmatch(s); m != nil {
		return Line{Kind: KindState, State: strings.ToLower(m[1])}, true
	}
	if m := reScore.FindStringSubmatch(s); m != nil {
		id, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return Line{}, false
		}
		return Line{Kind: KindScoreSaved, ScoreID: id}, true
	}
	if m := reReplay.FindStringSubmatch(s); m != nil {
		return Line{Kind: KindReplaySaved, ReplayPath: m[1]}, true
	}
	if reExit.MatchString(s) {
		return Line{Kind: KindExit}, true
	}
	return Line{}, false
}
