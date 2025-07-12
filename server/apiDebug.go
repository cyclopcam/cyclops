package server

import (
	"net/http"
	"time"

	"github.com/cyclopcam/cyclops/server/configdb"
	"github.com/cyclopcam/cyclops/server/eventdb"
	"github.com/cyclopcam/dbh"
	"github.com/cyclopcam/www"
	"github.com/julienschmidt/httprouter"
)

// Before you can use this API, you'll need to login.
// You can use cyclops --reset-user to create a local-only admin user.
// Then, login:
// curl -v -X POST -u user:password "http://localhost:8080/api/auth/login?loginMode=BearerToken"
// Save the bearer token from the response headers.

func (s *Server) httpDebugSendNotification(w http.ResponseWriter, r *http.Request, params httprouter.Params, user *configdb.User) {
	/*
		Example of arm/disarm:
		curl -X POST -H "Authorization: Bearer <token>" -d '{"eventType":"arm","detail":{"arm":{"userId":1,"deviceId":"phone-1"}}}' http://localhost:8080/api/debug/sendNotification
		curl -X POST -H "Authorization: Bearer <token>" -d '{"eventType":"disarm","detail":{"arm":{"userId":1,"deviceId":"phone-1"}}}' http://localhost:8080/api/debug/sendNotification

		Example of alarm trigger:
		curl -X POST -H "Authorization: Bearer <token>" -d '{"eventType":"alarm","detail":{"alarm":{"alarmType":"camera-object","cameraId":1}}}' http://localhost:8080/api/debug/sendNotification

		Example of panic button:
		curl -X POST -H "Authorization: Bearer <token>" -d '{"eventType":"alarm","detail":{"alarm":{"alarmType":"panic","cameraId":1}}}' http://localhost:8080/api/debug/sendNotification
	*/
	var event eventdb.Event
	www.ReadJSON(w, r, &event, 1024*1024)
	event.ID = 12345
	event.Time = dbh.MakeIntTime(time.Now())
	s.Notifications.InjectFakeEventIntoTransmitQueue(&event)
	www.SendOK(w)
}
