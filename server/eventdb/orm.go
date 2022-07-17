package eventdb

import (
	"github.com/bmharper/cyclops/server/dbh"
)

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

type Recording struct {
	BaseModel
	RandomID   string                 `json:"randomID"` // Used to ensure uniqueness when merging event databases
	StartTime  dbh.IntTime            `json:"startTime"`
	Format     string                 `json:"format"`           // Only valid value is "mp4"
	Labels     *dbh.JSONField[Labels] `json:"labels,omitempty"` // If labels is defined, then OntologyID is also defined
	OntologyID int64                  `json:"ontologyID,omitempty"`
}

func (r *Recording) VideoFilename() string {
	return r.baseFilename() + "." + r.Format
}

func (r *Recording) ThumbnailFilename() string {
	return r.baseFilename() + ".jpg"
}

func (r *Recording) baseFilename() string {
	t := r.StartTime.Get().UTC()
	return t.Format("2006-01/02/15-04-05-") + r.RandomID
}

// Labels associated with a recording
type Labels struct {
	VideoTags []int `json:"videoTags"` // Tags associated with the entire recording (eg "person"). Values refer to zero-based indices of OntologyDefinition.VideoTags
}

// An immutable ontology, referenced by a Recording, so that we know all the possible
// labels which were considered when a recording was labeled.
type Ontology struct {
	BaseModel
	CreatedAt  dbh.IntTime                        `json:"createdAt"`
	Definition *dbh.JSONField[OntologyDefinition] `json:"definition,omitempty"`
}

// Ontology spec
type OntologyDefinition struct {
	VideoTags []string `json:"videoTags"` // tags associated with the entire recording (eg ["person", "dog", "car"])
}
