package server

import (
	"net/http"
	"time"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/videodb"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpEventsGetTiles(w http.ResponseWriter, r *http.Request, _ httprouter.Params, user *configdb.User) {
	cameraID := www.RequiredQueryValue(r, "camera")
	level := www.RequiredQueryInt(r, "level")
	startIdx := www.QueryInt(r, "startIdx")
	endIdx := www.QueryInt(r, "endIdx")
	indices := www.QueryIntArray[uint32](r, "indices")
	if startIdx != 0 && len(indices) != 0 {
		www.PanicBadRequestf("Specify either indices or startIdx/endIdx, not both")
	}

	cam := s.getCameraFromIDOrPanic(cameraID)

	if level < 0 {
		level = 0
	} else if level > s.videoDB.MaxTileLevel() {
		level = s.videoDB.MaxTileLevel()
	}

	tileRequest := videodb.TileRequest{
		Level:    uint32(level),
		StartIdx: uint32(startIdx),
		EndIdx:   uint32(endIdx),
	}
	for _, i := range indices {
		tileRequest.Indices[i] = true
	}

	tiles, err := s.videoDB.ReadEventTiles(cam.LongLivedName(), tileRequest)
	www.Check(err)

	videoStartTime, err := s.videoDB.VideoStartTimeForCamera(cam.LongLivedName())
	if err != nil {
		// If there is no video footage, then don't return any tiles.
		tiles = []*videodb.EventTile{}
	}

	// Lookup the necessary internal IDs so that the caller doesn't have to make an additional API request for that.
	// This is things like 3 -> "person", 4 -> "car", etc.
	idToString := map[uint32]string{}
	for _, tile := range tiles {
		classIDs, err := videodb.GetClassIDsInTileBlob(tile.Tile)
		www.Check(err)
		for _, id := range classIDs {
			if _, ok := idToString[id]; !ok {
				str, err := s.videoDB.IDToString(id)
				www.Check(err)
				idToString[id] = str
			}
		}
	}

	// SYNC-GET-TILES-JSON
	response := struct {
		Tiles          []*videodb.EventTile `json:"tiles"`
		IDToString     map[uint32]string    `json:"idToString"`
		VideoStartTime int64                `json:"videoStartTime"` // This doesn't vary with the tiles being fetched, but it's useful data to side-load
	}{
		Tiles:          tiles,
		IDToString:     idToString,
		VideoStartTime: videoStartTime.UnixMilli(),
	}
	www.SendJSONOpt(w, &response, false)
}

func (s *Server) httpEventsGetDetails(w http.ResponseWriter, r *http.Request, _ httprouter.Params, user *configdb.User) {
	cameraID := www.RequiredQueryValue(r, "camera")
	startTime := time.UnixMilli(www.RequiredQueryInt64(r, "startTime"))
	endTime := time.UnixMilli(www.RequiredQueryInt64(r, "endTime"))
	cam := s.getCameraFromIDOrPanic(cameraID)

	events, err := s.videoDB.ReadEvents(cam.LongLivedName(), startTime, endTime)
	www.Check(err)

	// Get all the IDs so that the caller doesn't need to make an additional call
	// This is things like 3 -> "person", 4 -> "car", etc.
	idToString := map[uint32]string{}
	for _, ev := range events {
		if ev.Detections == nil {
			continue
		}
		for _, obj := range ev.Detections.Data.Objects {
			if idToString[obj.Class] == "" {
				idToString[obj.Class], err = s.videoDB.IDToString(obj.Class)
				www.Check(err)
			}
		}
	}

	// SYNC-GET-EVENT-DETAILS-JSON
	response := struct {
		Events     []*videodb.Event  `json:"events"`
		IDToString map[uint32]string `json:"idToString"`
	}{
		Events:     events,
		IDToString: idToString,
	}
	www.SendJSONOpt(w, &response, false)
}
