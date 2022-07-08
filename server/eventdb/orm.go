package eventdb

import "time"

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

type Recording struct {
	BaseModel
	RandomID  string    `json:"randomID"` // Used to ensure uniqueness when merging event databases
	StartTime time.Time `json:"startTime"`
	Format    string    `json:"format"` // Only valid value is "mp4"
}

func (r *Recording) Filename() string {
	t := r.StartTime.UTC()
	return t.Format("2006-01/02/15-04-05-") + r.RandomID + "." + r.Format
}
