package eventdb

import (
	"github.com/bmharper/cyclops/server/dbh"
	"github.com/bmharper/cyclops/server/defs"
)

// BaseModel is our base class for a GORM model.
// The default GORM Model uses int, but we prefer int64
type BaseModel struct {
	ID int64 `gorm:"primaryKey" json:"id"`
}

// A recording refers to either a high resolution video, a low resolution video, or both.
// When recording explicitly for training, we record just a low resolution video, because
// that's all we need.
// However, when recording a suspicious event, we want high resolution for playback and inspection,
// but we also want low resolution in case the user wants to turn that video clip into
// training data. This is why we record both high and low whenever the auto recorder kicks in.
type Recording struct {
	BaseModel
	RandomID     string                 `json:"randomID"`                                 // Used to ensure uniqueness when merging event databases
	StartTime    dbh.IntTime            `json:"startTime"`                                // Wall time when recording started
	FormatHD     string                 `json:"formatHD" gorm:"default:null"`             // Only valid value is "mp4"
	FormatLD     string                 `json:"formatLD" gorm:"default:null"`             // Only valid value is "mp4"
	Labels       *dbh.JSONField[Labels] `json:"labels,omitempty" gorm:"default:null"`     // If labels is defined, then OntologyID is also defined
	OntologyID   int64                  `json:"ontologyID,omitempty" gorm:"default:null"` // Labels reference indices in Ontology, which is why we need to store a reference to the Ontology
	Bytes        int64                  `json:"bytes"`                                    // total storage of videos + thumbnails
	DimensionsHD string                 `json:"dimensionsHD" gorm:"default:null"`         // width,height of HD video
	DimensionsLD string                 `json:"dimensionsLD" gorm:"default:null"`         // width,height of LD video
}

func (r *Recording) VideoFilename(res defs.Resolution) string {
	switch res {
	case defs.ResHD:
		return r.VideoFilenameHD()
	case defs.ResLD:
		return r.VideoFilenameLD()
	}
	panic("Invalid resolution '" + res + "'")
}

func (r *Recording) VideoFilenameHD() string {
	return r.baseFilename() + "-HD." + r.FormatHD
}

func (r *Recording) VideoFilenameLD() string {
	return r.baseFilename() + "-LD." + r.FormatLD
}

func (r *Recording) ThumbnailFilename() string {
	return r.baseFilename() + ".jpg"
}

func videoContentType(format string) string {
	switch format {
	case "mp4":
		return "video/mp4"
	}
	panic("Unrecognized video type " + format)
}

func (r *Recording) VideoContentType(res defs.Resolution) string {
	switch res {
	case defs.ResHD:
		return r.VideoContentTypeHD()
	case defs.ResLD:
		return r.VideoContentTypeLD()
	}
	panic("Invalid resolution '" + res + "'")
}

func (r *Recording) VideoContentTypeHD() string {
	return videoContentType(r.FormatHD)
}

func (r *Recording) VideoContentTypeLD() string {
	return videoContentType(r.FormatLD)
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
