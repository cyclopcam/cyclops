package dbh

import (
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

func IDListToString(ids []int64) string {
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

func IDListToSQLSet(ids []int64) string {
	// One might be tempted to return "(NULL)" here, for an empty set, but that seems
	// dangerous to me, in case the user unexpectedly has NULL entries in the database,
	// so we rather take the cautious route and allow an SQL error to occur.
	// It's the callers responsibility to ensure that 'ids' is not empty
	return "(" + IDListToString(ids) + ")"
}

func SanitizeIDList(s string) string {
	return IDListToString(StringToIDList(s))
}
