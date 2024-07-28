package server

import (
	"net/http"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/videodb"
	"github.com/julienschmidt/httprouter"
)

func (s *Server) httpEventsGetTiles(w http.ResponseWriter, r *http.Request, _ httprouter.Params, user *configdb.User) {
	cameraID := www.RequiredQueryValue(r, "camera")
	level := www.RequiredQueryInt(r, "level")
	startIdx := www.RequiredQueryInt(r, "startIdx")
	endIdx := www.RequiredQueryInt(r, "endIdx")

	cam := s.getCameraFromIDOrPanic(cameraID)

	if level < 0 {
		level = 0
	} else if level > s.videoDB.MaxTileLevel() {
		level = s.videoDB.MaxTileLevel()
	}

	tiles, err := s.videoDB.ReadEventTiles(cam.LongLivedName(), uint32(level), uint32(startIdx), uint32(endIdx))
	www.Check(err)

	// Lookup the necessary IDs so that the caller doesn't have to make an additional API request for that
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
		Tiles      []*videodb.EventTile `json:"tiles"`
		IDToString map[uint32]string    `json:"idToString"`
	}{
		Tiles:      tiles,
		IDToString: idToString,
	}
	www.SendJSONOpt(w, &response, false)
}
