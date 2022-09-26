package dbh

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

func StringToIDList(s string) []int64 {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) == 0 {
		return nil
	}
	r := []int64{}
	for _, p := range strings.Split(trimmed, ",") {
		i, _ := strconv.ParseInt(p, 10, 64)
		r = append(r, i)
	}
	return r
}

// I once mistakenly used this function instead of IDListToSQLSet, so that's why I'm making it private to this package.
// At the time of making that change, this function was never used outside of this package.
func idListToString(ids []int64) string {
	if len(ids) == 0 {
		return ""
	}
	b := strings.Builder{}
	for _, id := range ids {
		fmt.Fprintf(&b, "%v,", id)
	}
	s := b.String()
	return s[:len(s)-1]
}

// Returns a set of IDs with parens, e.g. "(1,2,3)"
// Note that the SQL drivers don't accept an SQL set as a positional argument (eg $1 or ?),
// so you need to bake it into your query string.
// See https://stackoverflow.com/questions/4788724/sqlite-bind-list-of-values-to-where-col-in-prm for an
// explanation of why it's not possible with SQLite, but presumably similar principles apply to other SQL interfaces.
func IDListToSQLSet(ids []int64) string {
	// One might be tempted to return "(NULL)" here, for an empty set, but that seems
	// dangerous to me, in case the user unexpectedly has NULL entries in the database,
	// so we rather take the cautious route and allow an SQL error to occur.
	// It's the callers responsibility to ensure that 'ids' is not empty
	return "(" + idListToString(ids) + ")"
}

func SanitizeIDList(s string) string {
	return idListToString(StringToIDList(s))
}

// Escape a byte array as a string literal, and return the entire literal, with quotes.
// eg. '\xDEADBEAF'
func PGByteArrayLiteral(b []byte) string {
	return "'\\x" + hex.EncodeToString(b) + "'"
}
