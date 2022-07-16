package dbh

import (
	"database/sql/driver"
	"time"
)

// IntTime makes it easy to save Int64 milliseconds into SQLite database with gorm
// In addition, it marshals nicely into JSON, and supports omitempty
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

func (i *IntTime) Scan(src any) error {
	if src == nil {
		*i = 0
		return nil
	}
	if srcInt, ok := src.(int32); ok {
		*i = IntTime(srcInt)
	} else if srcInt64, ok := src.(int64); ok {
		*i = IntTime(srcInt64)
	}
	return nil
}

func (i IntTime) Value() (driver.Value, error) {
	if i == 0 {
		return nil, nil
	}
	return int64(i), nil
}

/*
MilliTime is almost nice, but it doesn't support omitempty when marshalling to JSON

type MilliTime struct {
	// Embedding time.Time is better than making MilliTime an alias of time.Time, because embedding
	// brings in all the methods of time.Time, whereas an alias won't have any time-based methods on it.
	time.Time
}

func Milli(t time.Time) MilliTime {
	return MilliTime{t}
}

func (i *MilliTime) Scan(src any) error {
	if src == nil {
		*i = MilliTime{time.Time{}}
		return nil
	}
	if srcInt, ok := src.(int); ok {
		*i = MilliTime{time.UnixMilli(int64(srcInt))}
	} else if srcInt64, ok := src.(int64); ok {
		*i = MilliTime{time.UnixMilli(srcInt64)}
	}
	return nil
}

func (i MilliTime) Value() (driver.Value, error) {
	if i.IsZero() {
		return nil, nil
	}
	return i.UnixMilli(), nil
}

func (i MilliTime) MarshalJSON() ([]byte, error) {
	if i.IsZero() {
		return []byte("null"), nil
	}
	s := strconv.Itoa(int(i.UnixMilli()))
	//fmt.Printf("MarshalJSON(%v) UnixMilli = %v\n", i, i.UnixMilli())
	//return json.Marshal(i.UnixMilli())
	return []byte(s), nil
}

func (i *MilliTime) UnmarshalJSON(b []byte) error {
	var iv int64
	if err := json.Unmarshal(b, &iv); err != nil {
		return err
	}
	*i = MilliTime{time.UnixMilli(iv)}
	return nil
}
*/

/////////////////////////////////////////////////////////////////////////////

/*
MilliTime2 doesn't work, because you lose all the methods of time.Time

type MilliTime2 time.Time

func MakeMilliTime2(t time.Time) MilliTime2 {
	return MilliTime2(t)
}

func (i *MilliTime2) Scan(src any) error {
	if src == nil {
		*i = MilliTime2(time.Time{})
		return nil
	}
	if srcInt, ok := src.(int); ok {
		*i = MilliTime2(time.UnixMilli(int64(srcInt)))
	} else if srcInt64, ok := src.(int64); ok {
		*i = MilliTime2(time.UnixMilli(srcInt64))
	}
	return nil
}

func (i MilliTime2) Value() (driver.Value, error) {
	if time.Time(i).IsZero() {
		return nil, nil
	}
	return i.UnixMilli(), nil
}
*/
