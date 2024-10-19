package arc

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/cyclopcam/www"
)

// Package arc is a set of client-side functions for interacting with an Arc server.

type ArcServerCredentials struct {
	ServerUrl string // eg https://arc.cyclopcam.org (no trailing slash)
	ApiKey    string // secret key "sk-..."
}

func (a *ArcServerCredentials) IsConfigured() bool {
	return a.ServerUrl != "" && a.ApiKey != ""
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
// TODO: Fix this. It was originally written for our old 'eventdb' package, but it would
// need to change to use our new 'videodb' and 'fsv' packages.
// func UploadToArc(credentials *ArcServerCredentials, eventDBVideoRoot string, recording *eventdb.Recording, cameraName string) error {
func UploadToArc_FixMe(credentials *ArcServerCredentials, lowRes, highRes string, cameraName string) error {
	// Create the zip file
	zipBuf := bytes.Buffer{}
	zw := zip.NewWriter(&zipBuf)
	files := []struct {
		zipName  string
		diskName string
		compress bool
	}{
		//{"lowRes.mp4", filepath.Join(eventDBVideoRoot, recording.VideoFilenameLD()), false},
		//{"highRes.mp4", filepath.Join(eventDBVideoRoot, recording.VideoFilenameHD()), false},
		{"lowRes.mp4", lowRes, false},
		{"highRes.mp4", highRes, false},
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
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Authorization", "ApiKey "+credentials.ApiKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return errors.New("Upload failed: " + www.FailedRequestSummary(resp, err))
	}
	return nil
}
