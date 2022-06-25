package main

import (
	"os"
	"time"

	"github.com/bmharper/cyclops/server"
	"github.com/bmharper/cyclops/server/camera"
)

func main() {
	server := server.NewServer()

	ips := []string{
		//"192.168.10.27",
		"192.168.10.28",
		//"192.168.10.29",
	}

	for _, ip := range ips {
		base := "rtsp://admin:poortobydog123@" + ip + ":554"
		cam, err := camera.NewCamera("cam-"+ip, server.Log, camera.URLForHikVision(base, false), camera.URLForHikVision(base, true))
		if err != nil {
			panic(err)
		}
		server.AddCamera(cam)
	}
	err := server.StartAll()
	if err != nil {
		panic(err)
	}
	for i := 0; i < 1; i++ {
		time.Sleep(5 * time.Second)
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
			raw.SaveToMP4("dump/direct.mp4")
		}
	}
	time.Sleep(time.Second)
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
