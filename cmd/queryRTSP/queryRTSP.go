package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get and print informations about tracks published on a path.

func main() {
	c := gortsplib.Client{}

	u, err := url.Parse("rtsp://192.168.10.27:554")
	if err != nil {
		panic(err)
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	session, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	log.Printf("%v\n", session.Title)
	for _, media := range session.Medias {
		log.Printf("%v\n", media)
	}
}
