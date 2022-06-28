package main

import (
	"fmt"
	"os"
	"time"

	"github.com/bmharper/cyclops/server"
	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/config"
)

func main() {
	cfg, err := config.LoadConfig("")
	if err != nil {
		panic(err)
	}
	server := server.NewServer()
	err = server.LoadConfig(*cfg)
	if err != nil {
		panic(err)
	}

	err = server.StartAll()
	if err != nil {
		panic(err)
	}
	for i := 0; i < 5; i++ {
		time.Sleep(130 * time.Second)
		for icam, cam := range server.Cameras {
			server.Log.Infof("Dumping content %02d, camera %d", i, icam)
			//go extractCamera(server.Log, icam, cam)
			//extractCamera(server.Log, icam, cam)
			raw := cam.ExtractHighRes(camera.ExtractMethodClone)
			//raw.DumpBin("raw")

			os.Mkdir("dump", 0777)
			//filename := fmt.Sprintf("dump/%v-%02d.ts", cam.Name, i)
			//f, err := os.Create(filename)
			//if err != nil {
			//	panic(err)
			//}
			//if err := raw.SaveToMPEGTS(server.Log, f); err != nil {
			//	panic(err)
			//}
			//f.Close()
			fn := fmt.Sprintf("dump/%v-%02d.mp4", cam.Name, i)
			raw.SaveToMP4(fn)
		}
	}
	//time.Sleep(time.Second)
}

/*
func extractCamera(log log.Log, n int, cam *camera.Camera) {
	os.Mkdir("dump", 0770)
	filename := fmt.Sprintf("dump/%v-%02d.ts", cam.Name, n)
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	if err := cam.ExtractHighRes(f); err != nil {
		log.Errorf("Extraction failed: %v", err)
	}
	if err := f.Close(); err != nil {
		log.Errorf("Closing file failed: %v", err)
	}
}
*/
