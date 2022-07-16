package eventdb

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/bmharper/cyclops/server/dbh"
)

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

type Recording struct {
	BaseModel
	RandomID  string             `json:"randomID"` // Used to ensure uniqueness when merging event databases
	StartTime dbh.IntTime        `json:"startTime"`
	BarTime   dbh.IntTime        `json:"barTime,omitempty"`
	Format    string             `json:"format"`           // Only valid value is "mp4"
	Labels    *JSONField[Labels] `json:"labels,omitempty"` // JSON field
	//Labels    JSONField[Labels]   `json:"labels,omitempty"`  // JSON field
	//Labels2   *JSONField2[Labels] `json:"labels2,omitempty"` // JSON field
	//FooTime   dbh.MilliTime       `json:"fooTime,omitempty"`
}

func (r *Recording) Filename() string {
	t := r.StartTime.Get().UTC()
	return t.Format("2006-01/02/15-04-05-") + r.RandomID + "." + r.Format
}

// Labels associated with a recording
type Labels struct {
	Tags []string `json:"tags"`
}

/*
// JSONField wraps a plain old struct up so that it can be marshalled in and out of a GORM record
// You must include the struct tag `gorm:"type:bytes"` wherever you use a JSONField[]
// Note: In a future Go version, it may be possible to embed T into JSONField, which
// would get rid of the 'Data' element. As of Go 1.18, this is not possible.
// See https://github.com/golang/go/issues/49030#issuecomment-954336867 for original discussion prior to 1.18.
type JSONField[T any] struct {
	Data *T // This is your actual data. Go nil == SQL NULL
}

func MakeJSONField[T any](data *T) JSONField[T] {
	return JSONField[T]{
		Data: data,
	}
}

func (j *JSONField[T]) Scan(src any) error {
	if src == nil {
		j.Data = nil
		return nil
	}
	var val T
	srcByte, ok := src.([]byte)
	if !ok {
		return errors.New("JSONField underlying type must be []byte (some kind of Blob/JSON/JSONB field)")
	}
	if err := json.Unmarshal(srcByte, &val); err != nil {
		return err
	}
	j.Data = &val
	return nil
}

func (j JSONField[T]) Value() (driver.Value, error) {
	if j.Data == nil {
		return nil, nil
	}
	return json.Marshal(j.Data)
}

func (j JSONField[T]) MarshalJSON() ([]byte, error) {
	if j.Data == nil {
		return []byte("null"), nil
	}
	return json.Marshal(j.Data)
}

func (j *JSONField[T]) UnmarshalJSON(b []byte) error {
	var val T
	if err := json.Unmarshal(b, &val); err != nil {
		return err
	}
	j.Data = &val
	return nil
}
*/

/////////////////////////////////////////////////////////////

// I think I prefer JSONField2, because it supports omitempty in JSON output.
// It doesn't incur any more pointers than JSONField.

type JSONField[T any] struct {
	Data T
}

func MakeJSONField[T any](data T) *JSONField[T] {
	return &JSONField[T]{
		Data: data,
	}
}

func (j *JSONField[T]) Scan(src any) error {
	if src == nil {
		var empty T
		j.Data = empty
		return nil
	}
	srcByte, ok := src.([]byte)
	if !ok {
		return errors.New("JSONField underlying type must be []byte (some kind of Blob/JSON/JSONB field)")
	}
	if err := json.Unmarshal(srcByte, &j.Data); err != nil {
		return err
	}
	return nil
}

func (j JSONField[T]) Value() (driver.Value, error) {
	return json.Marshal(j.Data)
}

func (j JSONField[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Data)
}

func (j *JSONField[T]) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		// According to docs, this is a no-op by convention
		//var empty T
		//j.Data = empty
		return nil
	}
	if err := json.Unmarshal(b, &j.Data); err != nil {
		return err
	}
	return nil
}
