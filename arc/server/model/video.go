package model

import "time"

// SYNC-ARC-VIDEO-RECORD
type Video struct {
	BaseModel
	CreatedBy  int64     `json:"createdBy"`
	CreatedAt  time.Time `json:"createdAt"`
	CameraName string    `json:"cameraName"` // Whatever the user chose to name the camera
}
