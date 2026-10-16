package scores

import (
	"database/sql"
	"strings"
)

// extraColumn is an optional Map column: older databases may lack it, then def is read instead.
type extraColumn struct{ name, def string }

var extraColumns = [...]extraColumn{
	{"Creator", "''"}, {"Bpm", "0"}, {"SongLength", "0"}, {"RegularNoteCount", "0"}, {"LongNoteCount", "0"},
	{"MineCount", "0"}, {"RankedStatus", "-1"}, {"TimesPlayed", "0"}, {"MapId", "0"}, {"MapSetId", "0"},
}

// extraFallback reads every extra column as its default, for a database whose columns are not known yet.
var extraFallback = func() string {
	defs := make([]string, len(extraColumns))
	for i, c := range extraColumns {
		defs[i] = c.def
	}
	return strings.Join(defs, ", ")
}()

// extraProbe returns the SQL that reads the extra map columns, defaults standing in for missing ones.
func extraProbe(db *sql.DB) (string, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info('Map')`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return "", err
		}
		have[n] = true
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	exprs := make([]string, len(extraColumns))
	for i, c := range extraColumns {
		exprs[i] = c.def
		if have[c.name] {
			exprs[i] = "COALESCE(m." + c.name + "," + c.def + ")"
		}
	}
	return strings.Join(exprs, ", "), nil
}

// mapExtra holds the optional map columns as read.
type mapExtra struct {
	creator              string
	bpm                  float64
	length               int64
	regular, long, mines int
	ranked, played       int
	mapID, setID         int64
}

// dests lists the scan targets in the order of extraColumns.
func (e *mapExtra) dests() []any {
	return []any{&e.creator, &e.bpm, &e.length, &e.regular, &e.long, &e.mines, &e.ranked, &e.played, &e.mapID, &e.setID}
}

func (e mapExtra) apply(m *Map) {
	m.Creator, m.BPM, m.LengthMS = e.creator, e.bpm, e.length
	m.RegularNotes, m.LongNotes, m.Mines, m.TimesPlayed = e.regular, e.long, e.mines, e.played
	m.MapID, m.MapSetID = e.mapID, e.setID
	m.RankedStatus = rankedName(e.ranked)
}

// rankedName maps Quaver's RankedStatus integer; anything else, or a missing column, is "unknown".
func rankedName(v int) string {
	switch v {
	case 0:
		return "not_submitted"
	case 1:
		return "unranked"
	case 2:
		return "ranked"
	case 3:
		return "dan"
	}
	return "unknown"
}
