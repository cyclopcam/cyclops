package dbh

import (
	"time"
)

// IntTime makes it easy to save Int64 milliseconds into SQLite database with gorm
// Perhaps there is a way to do this using time.Time as the datatype on your model, but I couldn't figure it out.
type IntTime int64

func MakeIntTime(v time.Time) IntTime {
	return IntTime(v.UnixMilli())
}

func (t *IntTime) Set(v time.Time) {
	*t = IntTime(v.UnixMilli())
}

func (t *IntTime) Get() time.Time {
	return time.UnixMilli(int64(*t)).UTC()
}
