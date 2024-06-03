package main

import (
	"database/sql"
	"fmt"

	"github.com/cyclopcam/cyclops/server/scanner"
	"github.com/cyclopcam/cyclops/server/videodb"
)

func cameraScan() {
	cams, err := scanner.ScanForLocalCameras(nil)
	if err != nil {
		panic(err)
	}
	for _, c := range cams {
		fmt.Printf("Camera: %v\n", c)
	}
}

func dumpTiles() {
	//logs, _ := log.NewLog()
	//vdb, _ := videodb.NewVideoDB(logs, "/home/ben/cyclops2")
	//db, err := dbh.OpenDB(logs, dbh.MakeSqliteConfig("/home/ben/cyclops2/videos.sqlite"), nil, 0)
	db, err := sql.Open("sqlite3", "/home/ben/cyclops2/videos.sqlite")
	if err != nil {
		panic(err)
	}
	//rows, _ := db.Raw("select tile from event_tile").Rows()
	rows, _ := db.Query("select tile from event_tile")
	for rows.Next() {
		blob := []byte{}
		rows.Scan(&blob)
		lines := videodb.DecompressTileToRawLines(blob)
		for _, line := range lines {
			fmt.Printf("%x\n", line)
		}
	}
}

func main() {
	//cameraScan()
	dumpTiles()
}
