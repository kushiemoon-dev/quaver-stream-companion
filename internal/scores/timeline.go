package scores

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
)

const (
	maxBreakdown = 100000
	maxPoints    = 200
	maxMisses    = 500
	missDigit    = 5
)

// ErrNoTimeline is returned when a score has no usable judgment sequence.
var ErrNoTimeline = errors.New("no judgment timeline for this score")

// weights are Quaver's standard accuracy weights per judgment digit: Marvelous, Perfect,
// Great, Good, Okay, Miss. Summed over a play and divided by its judgment count they
// reproduce the accuracy stored in quaver.db.
var weights = [6]float64{100, 98.25, 65, 25, -100, -50}

// Timeline is the judgment sequence of a play as an accuracy curve.
// Points are the accuracy in percent after each segment of judgments, Counts the
// judgments of each segment, and Misses the indexes of missed judgments.
type Timeline struct {
	ScoreID int64     `json:"score_id"`
	Total   int       `json:"total"`
	Points  []float64 `json:"points"`
	Misses  []int     `json:"misses"`
	Counts  [][6]int  `json:"counts"`
}

// DecodeBreakdown decodes a JudgementBreakdown value into one digit (0 to 5) per judgment.
func DecodeBreakdown(s string) ([]byte, error) {
	if len(s) == 0 || len(s) > base64.StdEncoding.EncodedLen(maxBreakdown+1024) {
		return nil, errors.New("breakdown has an unusable size")
	}
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil || len(raw) < 4 {
		return nil, errors.New("breakdown is not valid base64")
	}
	n := binary.LittleEndian.Uint32(raw)
	if n == 0 || n > maxBreakdown {
		return nil, errors.New("breakdown length is out of range")
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw[4:]))
	if err != nil {
		return nil, errors.New("breakdown is not gzip data")
	}
	out, err := io.ReadAll(io.LimitReader(zr, int64(n)+1))
	if err != nil || len(out) != int(n) {
		return nil, errors.New("breakdown length does not match its data")
	}
	for i, c := range out {
		if c < '0' || c > '5' {
			return nil, errors.New("breakdown has an unknown judgment")
		}
		out[i] = c - '0'
	}
	return out, nil
}

// BuildTimeline turns judgment digits into a timeline of at most 200 points.
func BuildTimeline(scoreID int64, seq []byte) Timeline {
	total := len(seq)
	segs := min(total, maxPoints)
	t := Timeline{
		ScoreID: scoreID, Total: total,
		Points: make([]float64, 0, segs), Counts: make([][6]int, 0, segs), Misses: []int{},
	}
	sum, start := 0.0, 0
	for i := 1; i <= segs; i++ {
		end := i * total / segs
		var c [6]int
		for _, d := range seq[start:end] {
			c[d]++
			sum += weights[d]
		}
		t.Points = append(t.Points, math.Round(math.Max(sum/float64(end), 0)*100)/100)
		t.Counts = append(t.Counts, c)
		start = end
	}
	var misses []int
	for i, d := range seq {
		if d == missDigit {
			misses = append(misses, i)
		}
	}
	for i := 0; i < len(misses) && len(t.Misses) < maxMisses; i++ {
		t.Misses = append(t.Misses, misses[i*len(misses)/min(len(misses), maxMisses)])
	}
	return t
}

// Timeline reads the judgment timeline of a score. It returns ErrNotFound for an
// unknown score and ErrNoTimeline when the stored sequence is missing or unreadable.
func (s *Store) Timeline(ctx context.Context, id int64) (Timeline, error) {
	t, err := s.timeline(ctx, id)
	return t, s.track(err)
}

func (s *Store) timeline(ctx context.Context, id int64) (Timeline, error) {
	var raw string
	err := s.api.QueryRowContext(ctx,
		`SELECT COALESCE(JudgementBreakdown,'') FROM Score WHERE Id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Timeline{}, ErrNotFound
	}
	if err != nil {
		if strings.Contains(err.Error(), "no such column") {
			return Timeline{}, ErrNoTimeline
		}
		return Timeline{}, fmt.Errorf("read judgments of score %d: %w", id, err)
	}
	seq, err := DecodeBreakdown(raw)
	if err != nil {
		return Timeline{}, ErrNoTimeline
	}
	return BuildTimeline(id, seq), nil
}
