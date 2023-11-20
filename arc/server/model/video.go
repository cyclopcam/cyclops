package model

import (
	"github.com/cyclopcam/cyclops/pkg/dbh"
)

// SYNC-ARC-VIDEO-RECORD
type Video struct {
	BaseModel
	CreatedBy  int64         `json:"createdBy"`
	CreatedAt  dbh.MilliTime `json:"createdAt"`
	CameraName string        `json:"cameraName"` // Whatever the user chose to name the camera
	HasLabels  bool          `json:"hasLabels"`  // True if the video has been labeled
}
