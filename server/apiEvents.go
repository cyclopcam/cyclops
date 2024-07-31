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
		Tiles      []*videodb.EventTile `json:"tiles"`
		IDToString map[uint32]string    `json:"idToString"`
	}{
		Tiles:      tiles,
		IDToString: idToString,
	}
	www.SendJSONOpt(w, &response, false)
}
