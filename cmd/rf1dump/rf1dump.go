package main

import (
	"fmt"
	"os"

	"github.com/cyclopcam/cyclops/pkg/videoformat/rf1"
)

// Dump info about an RF1 video file

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	filename := os.Args[1]

	file, err := rf1.Open(filename, rf1.OpenModeReadOnly)
	check(err)
	defer file.Close()

	track := file.Tracks[0]
	size := track.Count()
	nalu, err := track.ReadIndex(0, size)
	check(err)

	colors := []string{"\033[31m", "\033[32m"}
	icolor := 0
	resetColor := "\033[0m"
	lastPTS := int64(0)
	for i := 0; i < size; i++ {
		n := nalu[i]
		flags := formatFlags(n.Flags)
		pts := n.PTS.Sub(track.TimeBase).Milliseconds()
		if pts != lastPTS {
			icolor = (icolor + 1) % len(colors)
			lastPTS = pts
		}
		fmt.Printf("%v%v: %v %v %v\n", colors[icolor], i, flags, pts, n.Length)
	}
	fmt.Printf("%v", resetColor)
}

func formatFlags(flags rf1.IndexNALUFlags) string {
	flagsStr := ""
	if flags&rf1.IndexNALUFlagKeyFrame != 0 {
		flagsStr += "K"
	} else {
		flagsStr += " "
	}
	if flags&rf1.IndexNALUFlagEssentialMeta != 0 {
		flagsStr += "E"
	} else {
		flagsStr += " "
	}
	if flags&rf1.IndexNALUFlagAnnexB != 0 {
		flagsStr += "B"
	} else {
		flagsStr += " "
	}
	return flagsStr
}
