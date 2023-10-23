package iox

import (
	"io"
	"os"
)

func WriteStreamToFile(dstFilename string, src io.Reader) error {
	dstFile, err := os.Create(dstFilename)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, src)
	if err != nil {
		os.Remove(dstFilename)
		return err
	}
	return nil
}
