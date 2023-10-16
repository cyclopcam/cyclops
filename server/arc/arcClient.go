package arc

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cyclopcam/cyclops/pkg/www"
	"github.com/cyclopcam/cyclops/server/eventdb"
)

// Package arc is a set of client-side functions for interacting with an Arc server.

type ArcServerCredentials struct {
	ServerUrl string // eg https://arc.cyclopcam.org (no trailing slash)
	Username  string
	Password  string
}

func (a *ArcServerCredentials) IsConfigured() bool {
	return a.ServerUrl != "" && a.Username != "" && a.Password != ""
}

func addFileToZip(zw *zip.Writer, filenameInZip, filenameOnDisk string, compress bool) error {
	src, err := os.Open(filenameOnDisk)
	if err != nil {
		return err
	}
	defer src.Close()
	method := zip.Deflate
	if !compress {
		method = zip.Store
	}
	header := &zip.FileHeader{
		Name:   filenameInZip,
		Method: method,
	}
	f, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		return err
	}
	return nil
}

// Share the recording with an Arc server.
func UploadToArc(credentials *ArcServerCredentials, eventDBVideoRoot string, recording *eventdb.Recording, cameraName string) error {
	// Create the zip file
	zipBuf := bytes.Buffer{}
	zw := zip.NewWriter(&zipBuf)
	files := []struct {
		zipName  string
		diskName string
		compress bool
	}{
		{"lowRes.mp4", filepath.Join(eventDBVideoRoot, recording.VideoFilenameLD()), false},
		{"highRes.mp4", filepath.Join(eventDBVideoRoot, recording.VideoFilenameHD()), false},
	}
	for _, file := range files {
		if err := addFileToZip(zw, file.zipName, file.diskName, file.compress); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	// Upload the zip file
	query := www.EncodeQuery(map[string]string{"cameraName": cameraName})
	req, err := http.NewRequest("PUT", credentials.ServerUrl+"/api/video?"+query, &zipBuf)
	if err != nil {
		return err
	}
	req.SetBasicAuth(credentials.Username, credentials.Password)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("Upload failed: " + www.FailedRequestSummary(resp, err))
	}
	return nil
}
