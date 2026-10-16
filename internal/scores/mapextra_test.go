package scores

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/testutil"
)

const extrasUpdate = `UPDATE Map SET Creator = 'Mapper', Bpm = 172.5, SongLength = 95000, RegularNoteCount = 800, LongNoteCount = 120, MineCount = 3, RankedStatus = 2, TimesPlayed = 7, MapId = 1234, MapSetId = 567`

func TestMapCarriesTheExtraColumns(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "T", Mode: 1, LastPlayedMS: 1})
	d.Exec(t, extrasUpdate)
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 10, Grade: 7})
	st := open(t, d)

	want := Map{Creator: "Mapper", BPM: 172.5, LengthMS: 95000, RegularNotes: 800, LongNotes: 120, Mines: 3,
		RankedStatus: "ranked", TimesPlayed: 7, MapID: 1234, MapSetID: 567}
	check := func(what string, m Map) {
		t.Helper()
		m.MD5, m.Title, m.Artist, m.DifficultyName, m.Difficulty, m.Keys, m.Scratch, m.LastPlayedMS = "", "", "", "", 0, 0, false, 0
		if m != want {
			t.Errorf("%s: %+v, want %+v", what, m, want)
		}
	}
	m, err := st.LastPlayedMap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	check("last played map", m)
	r, err := st.ScoreByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	check("score map", r.Map)
}

func TestRankedStatusNames(t *testing.T) {
	for v, want := range map[int]string{0: "not_submitted", 1: "unranked", 2: "ranked", 3: "dan", 4: "unknown", -1: "unknown", 99: "unknown"} {
		if got := rankedName(v); got != want {
			t.Errorf("rankedName(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestOlderDatabaseWithoutTheExtraColumnsStillReads(t *testing.T) {
	d := testutil.NewOldMapQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "Old", Mode: 1, LastPlayedMS: 1})
	id := d.AddScore(t, testutil.ScoreRow{MapMD5: "m", Total: 10, Grade: 7})
	st := open(t, d)

	m, err := st.LastPlayedMap(context.Background())
	if err != nil || m.Title != "Old" || m.Creator != "" || m.BPM != 0 || m.LengthMS != 0 || m.MapID != 0 || m.RankedStatus != "unknown" {
		t.Fatalf("map = %+v, err %v", m, err)
	}
	r, err := st.ScoreByID(context.Background(), id)
	if err != nil || r.Map.Title != "Old" || r.Map.RankedStatus != "unknown" {
		t.Fatalf("score map = %+v, err %v", r.Map, err)
	}
}

func TestNullExtraColumnsReadAsDefaults(t *testing.T) {
	d := testutil.NewQuaverDir(t)
	d.AddMap(t, testutil.MapRow{MD5: "m", Title: "Null", Mode: 1, LastPlayedMS: 1})
	m, err := open(t, d).LastPlayedMap(context.Background())
	if err != nil || m.Creator != "" || m.BPM != 0 || m.RegularNotes != 0 || m.RankedStatus != "unknown" {
		t.Fatalf("map = %+v, err %v", m, err)
	}
}

func TestMapJSONGolden(t *testing.T) {
	m := Map{MD5: "abc", Artist: "A", Title: "T", DifficultyName: "Hard", Difficulty: 18.5, Keys: 4, LastPlayedMS: 5,
		Creator: "Mapper", BPM: 172.5, LengthMS: 95000, RegularNotes: 800, LongNotes: 120, Mines: 3,
		RankedStatus: "ranked", TimesPlayed: 7, MapID: 1234, MapSetID: 567}
	got, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"md5":"abc","artist":"A","title":"T","difficulty_name":"Hard","difficulty":18.5,"keys":4,"scratch":false,"last_played_ms":5,` +
		`"creator":"Mapper","bpm":172.5,"length_ms":95000,"regular_notes":800,"long_notes":120,"mines":3,"ranked_status":"ranked","times_played":7,"map_id":1234,"map_set_id":567}`
	if string(got) != want {
		t.Fatalf("json = %s\nwant   %s", got, want)
	}
}

func TestExtrasFallbackBeforeTheColumnsAreKnown(t *testing.T) {
	f := scratchFlag{probe: func(*sql.DB) (string, error) { return "", context.DeadlineExceeded }, fallback: extraFallback}
	if got := f.expr(nil, time.Now()); got != extraFallback {
		t.Fatalf("got %q", got)
	}
}
