package server

import (
	"net/http"
	"os"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/julienschmidt/httprouter"
)

// example: curl -H "Authorization: Bearer h1cPWbUyCKBeEPc8NgW8Fj4q+TpgRUIuvezTr0NFV80=" http://localhost:8080/api/train/getDataset
func (s *Server) httpTrainGetDataset(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	tmp, err := os.CreateTemp("", "training.zip")
	www.Check(err)
	defer os.Remove(tmp.Name())

	if err := s.train.GetDataset(tmp); err != nil {
		www.PanicServerError(err.Error())
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=training.zip")
	http.ServeFile(w, r, tmp.Name())
}
