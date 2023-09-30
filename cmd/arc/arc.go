package main

import (
	"fmt"
	"os"

	"github.com/akamensky/argparse"
	"github.com/bmharper/cyclops/arc/server"
)

func main() {
	parser := argparse.NewParser("arc", "Store of training videos for cyclops camera system")
	hotReloadWWW := parser.Flag("", "hot", &argparse.Options{Help: "Hot reload www instead of embedding into binary", Default: false})
	configFilePath := parser.String("c", "config", &argparse.Options{Help: "Config file path", Default: "arc.json"})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
		os.Exit(1)
	}

	s, err := server.NewServer(*configFilePath)
	if err != nil {
		panic(err)
	}
	s.HotReloadWWW = *hotReloadWWW
	s.ListenForInterruptSignal()
	s.ListenHTTP(":8081")
}
