package video

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/arc/server/auth"
	"github.com/cyclopcam/cyclops/arc/server/model"
	"github.com/cyclopcam/cyclops/arc/server/storage"
	"github.com/cyclopcam/cyclops/arc/server/storagecache"
	"github.com/cyclopcam/cyclops/pkg/dbh"
	"github.com/cyclopcam/cyclops/pkg/iox"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/rando"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/julienschmidt/httprouter"
	"gorm.io/gorm"
)

// Note that the video filenames in blob storage are also represented in code form
// in video.ts. So if you change a path such as /:id/lowRes.mp4, then don't forget
// to also change it in video.ts

type VideoServer struct {
	log          log.Log
	db           *gorm.DB
	storage      storage.Storage
	storageCache *storagecache.StorageCache
}

func NewVideoServer(log log.Log, db *gorm.DB, storage storage.Storage, storageCache *storagecache.StorageCache) *VideoServer {
	return &VideoServer{
		log:          log,
		db:           db,
		storage:      storage,
		storageCache: storageCache,
	}
}

func videoFilename(vidID int64, file string) string {
	return fmt.Sprintf("videos/%v/%v", vidID, file)
}

// Upload a video.
// A video is a zip file containing the following files:
// - lowRes.mp4
// - highRes.mp4
// The low res video should be from the low res stream of the camera,
// so that we can train on the exact same video that we are using for inference.
func (s *VideoServer) HttpPutVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	s.log.Infof("Video incoming")
	maxSize := int64(16 * 1024 * 1024)
	if r.ContentLength > maxSize {
		www.PanicBadRequestf("Request body is too large: %v. Maximum size: %v MB", r.ContentLength, maxSize/(1024*1024))
	}
	cameraName := strings.TrimSpace(www.RequiredQueryValue(r, "cameraName"))
	if len(cameraName) > 200 {
		cameraName = cameraName[:200]
	}
	reader := io.LimitReader(r.Body, maxSize)
	body, err := io.ReadAll(reader)
	www.Check(err)
	zipReader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	www.Check(err)

	lowResTempFile, lowResReader, err := extractZipFile(zipReader, "lowRes.mp4", maxSize)
	www.Check(err)
	defer os.Remove(lowResTempFile)
	defer lowResReader.Close()

	highResTempFile, highResReader, err := extractZipFile(zipReader, "highRes.mp4", maxSize)
	www.Check(err)
	defer os.Remove(highResTempFile)
	defer highResReader.Close()

	mediumResTempFile := rando.TempFilename(".mp4")
	defer os.Remove(mediumResTempFile)
	www.Check(videox.TranscodeMediumQualitySeekable(highResTempFile, mediumResTempFile))
	mediumResReader, err := os.Open(mediumResTempFile)
	www.Check(err)
	defer mediumResReader.Close()

	// Create thumbnail
	highResDuration, err := videox.ExtractVideoDuration(highResTempFile)
	www.Check(err)
	thumbnail, err := videox.ExtractFrame(highResTempFile, highResDuration.Seconds()/2, 1280)
	www.Check(err)

	vid := model.Video{
		CreatedBy:  cred.UserID,
		CreatedAt:  dbh.Milli(time.Now().UTC()),
		CameraName: cameraName,
	}
	tx := s.db.Begin()
	www.Check(tx.Error)
	defer tx.Rollback()
	www.Check(tx.Create(&vid).Error)
	www.Check(storage.WriteFile(s.storage, videoFilename(vid.ID, "lowRes.mp4"), lowResReader))
	www.Check(storage.WriteFile(s.storage, videoFilename(vid.ID, "mediumRes.mp4"), mediumResReader))
	www.Check(storage.WriteFile(s.storage, videoFilename(vid.ID, "highRes.mp4"), highResReader))
	www.Check(storage.WriteFile(s.storage, videoFilename(vid.ID, "thumb.jpg"), bytes.NewReader(thumbnail)))
	www.Check(tx.Commit().Error)
	www.SendID(w, vid.ID)
	s.log.Infof("New video %v from user %v, camera %v", vid.ID, cred.UserID, cameraName)
}

// Extract a single file from a zip file.
// Return the name of the temporary extract location, and a reader on that temporary file.
func extractZipFile(zf *zip.Reader, filename string, maxBytes int64) (string, io.ReadCloser, error) {
	content, err := zf.Open(filename)
	if err != nil {
		www.PanicBadRequestf("Failed to open %v in zip file: %v", filename, err)
	}
	defer content.Close()
	stat, err := content.Stat()
	if err != nil {
		return "", nil, err
	}
	if stat.Size() > maxBytes {
		return "", nil, fmt.Errorf("%v is too large: %v", filename, stat.Size())
	}
	tempFile := rando.TempFilename(filepath.Ext(filename))
	err = iox.WriteStreamToFile(tempFile, content)
	if err != nil {
		return "", nil, err
	}
	reader, err := os.Open(tempFile)
	if err != nil {
		os.Remove(tempFile)
		return "", nil, err
	}
	return tempFile, reader, nil
}

func (s *VideoServer) getVideoOrPanic(id string, cred *auth.Credentials) *model.Video {
	id64, _ := strconv.ParseInt(id, 10, 64)
	vid := model.Video{}
	www.Check(s.db.First(&vid, id64).Error)
	if !cred.IsAdmin() && vid.CreatedBy != cred.UserID {
		www.PanicForbiddenf("You are not allowed to access this video")
	}
	return &vid
}

func (s *VideoServer) HttpVideoThumbnail(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	vid := s.getVideoOrPanic(params.ByName("id"), cred)
	file, err := s.storage.ReadFile(videoFilename(vid.ID, "thumb.jpg"))
	www.Check(err)
	defer file.Reader.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	io.Copy(w, file.Reader)
}

func (s *VideoServer) HttpGetVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	res := params.ByName("res")
	if res != "low" && res != "medium" && res != "high" {
		www.PanicBadRequestf("Invalid resolution: %v. Valid values are 'low', 'medium', 'high'", res)
	}
	//seekableUrl := www.QueryValue(r, "seekableUrl") == "1"
	vid := s.getVideoOrPanic(params.ByName("id"), cred)
	var reader io.ReadCloser
	if s.storageCache != nil {
		// Assume that the underlying storage system is a blob store that is a PITA to randomly seek
		// Instead of a cache, we could also use signed URLs (https://cloud.google.com/storage/docs/access-control/signing-urls-with-helpers#storage-signed-url-object-go)
		file, err := s.storageCache.Open(videoFilename(vid.ID, res+"Res.mp4"))
		www.Check(err)
		reader = file
	} else {
		file, err := s.storage.ReadFile(videoFilename(vid.ID, res+"Res.mp4"))
		www.Check(err)
		reader = file.Reader
	}
	defer reader.Close()
	w.Header().Set("Content-Type", "video/mp4")
	if seeker, ok := reader.(io.ReadSeeker); ok {
		http.ServeContent(w, r, "video.mp4", vid.CreatedAt.Time, seeker)
	} else {
		// This ends up creating a poorer html <video> element experience
		io.Copy(w, reader)
	}
}

func (s *VideoServer) HttpListVideos(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
	vids := []model.Video{}
	q := s.db
	if !cred.IsAdmin() {
		q = q.Where("created_by = ?", cred.UserID)
	}
	www.Check(q.Find(&vids).Error)
	www.SendJSON(w, vids)
}
