package main

import (
	"time"

	"github.com/bmharper/cyclops/server"
	"github.com/bmharper/cyclops/server/camera"
)

func main() {
	server := server.NewServer()

	ips := []string{
		"192.168.10.27",
		"192.168.10.28",
		"192.168.10.29",
	}

	for _, ip := range ips {

		base := "rtsp://admin:poortobydog123@" + ip + ":554"
		cam, err := camera.NewCamera("cam "+ip, server.Log, camera.URLForHikVision(base, false), camera.URLForHikVision(base, true))
		if err != nil {
			panic(err)
		}
		server.AddCamera(cam)
	}
	err := server.StartAll()
	if err != nil {
		panic(err)
	}
	time.Sleep(5 * time.Second)

	/*
		name := "driveway"
		high := "rtsp://admin:poortobydog123@192.168.10.27:554/Streaming/Channels/101"
		low := "rtsp://admin:poortobydog123@192.168.10.27:554/Streaming/Channels/102"

		log, err := log.NewLog()
		if err != nil {
			panic(fmt.Errorf("Error opening log: %w", err))
		}

		cam, err := camera.NewCamera(name, log, low, high)
		if err != nil {
			panic(fmt.Errorf("Error opening camera: %w", err))
		}

		if err := cam.Start(); err != nil {
			panic(fmt.Errorf("Error starting camera %v: %w", cam.Name, err))
		}

		//if err := cam.LowRes.Client.Wait(); err != nil {
		//	log.Errorf("LowRes error %v", err)
		//}
		time.Sleep(5 * time.Second)
		cam.Close()
	*/
}
