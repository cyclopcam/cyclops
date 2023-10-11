package video

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/arc/server/auth"
	"github.com/cyclopcam/cyclops/arc/server/model"
	"github.com/cyclopcam/cyclops/arc/server/storage"
	"github.com/cyclopcam/cyclops/arc/server/storagecache"
	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/rando"
	"github.com/cyclopcam/cyclops/pkg/videox"
	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/julienschmidt/httprouter"
	"gorm.io/gorm"
)

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

func (s *VideoServer) HttpPutVideo(w http.ResponseWriter, r *http.Request, params httprouter.Params, cred *auth.Credentials) {
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
	lowRes, err := zipReader.Open("lowRes.mp4")
	if err != nil {
		www.PanicBadRequestf("Failed to open lowRes.mp4 in zip file: %v", err)
	}
	defer lowRes.Close()
	lowResStat, err := lowRes.Stat()
	www.Check(err)
	if lowResStat.Size() > maxSize {
		www.PanicBadRequestf("lowRes.mp4 is too large: %v", lowResStat.Size())
	}

	highRes, err := zipReader.Open("highRes.mp4")
	if err != nil {
		www.PanicBadRequestf("Failed to open highRes.mp4 in zip file: %v", err)
	}
	defer highRes.Close()
	highResStat, err := highRes.Stat()
	www.Check(err)
	if highResStat.Size() > maxSize {
		www.PanicBadRequestf("highRes.mp4 is too large: %v", highResStat.Size())
	}

	lowResBytes, err := io.ReadAll(lowRes)
	www.Check(err)
	lowResTempfile := rando.TempFilename(".jpg")
	defer os.Remove(lowResTempfile)
	www.Check(os.WriteFile(lowResTempfile, lowResBytes, 0644))
	lowResDuration, err := videox.ExtractVideoDuration(lowResTempfile)
	www.Check(err)
	thumbnail, err := videox.ExtractFrame(lowResTempfile, lowResDuration.Seconds()/2)
	www.Check(err)

	vid := model.Video{
		CreatedBy:  cred.UserID,
		CreatedAt:  time.Now().UTC(),
		CameraName: cameraName,
	}
	tx := s.db.Begin()
	www.Check(tx.Error)
	defer tx.Rollback()
	www.Check(tx.Create(&vid).Error)
	www.Check(storage.WriteFile(s.storage, videoFilename(vid.ID, "lowRes.mp4"), bytes.NewReader(lowResBytes)))
	www.Check(storage.WriteFile(s.storage, videoFilename(vid.ID, "highRes.mp4"), highRes))
	www.Check(storage.WriteFile(s.storage, videoFilename(vid.ID, "thumb.jpg"), bytes.NewReader(thumbnail)))
	www.Check(tx.Commit().Error)
	www.SendID(w, vid.ID)
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
	if res != "low" && res != "high" {
		www.PanicBadRequestf("Invalid resolution: %v. Valid values are 'low' and 'high'", res)
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
		http.ServeContent(w, r, "video.mp4", vid.CreatedAt, seeker)
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
