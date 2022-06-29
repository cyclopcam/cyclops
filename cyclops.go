package main

import (
	"fmt"
	"os"
	"time"

	"github.com/bmharper/cyclops/server"
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/config"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	cfg, err := config.LoadConfig("")
	check(err)
	srv := server.NewServer()
	check(srv.LoadConfig(*cfg))
	check(srv.StartAll())

	//dumpCameras(srv)
	srv.SetupHTTP(":8080")
}

func dumpCameras(srv *server.Server) {
	for i := 0; i < 5; i++ {
		time.Sleep(130 * time.Second)
		for icam, cam := range srv.Cameras {
			srv.Log.Infof("Dumping content %02d, camera %d", i, icam)
			raw, _ := cam.ExtractHighRes(camera.ExtractMethodClone, 5*time.Second)
			//raw.DumpBin("raw")

			os.Mkdir("dump", 0777)
			fn := fmt.Sprintf("dump/%v-%02d.mp4", cam.Name, i)
			raw.SaveToMP4(fn)
		}
	}
}
