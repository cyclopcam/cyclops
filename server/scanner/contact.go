package scanner

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/cyclopcam/cyclops/server/camera"
)

type ScanMethod int

const (
	ScanMethodHTTP ScanMethod = 1 << iota
	ScanMethodRTSP
)

// Try to contact the camera, using whatever network heuristics you specify in scanMethods
func TryToContactCamera(host string, timeout time.Duration, scanMethods ScanMethod) (camera.CameraBrands, error) {
	//fmt.Printf("Contacting %v...\n", ip)

	// 100ms is usually sufficient on my home network with HikVision cameras and ethernet, but it might be too aggressive for some.
	// This is controllable from the app, and each time the user hits "scan again", it raises the timeout.
	if timeout == 0 {
		timeout = 100 * time.Millisecond
	}

	enableHTTP := scanMethods&ScanMethodHTTP != 0
	enableRTSP := scanMethods&ScanMethodRTSP != 0

	nMethods := 0
	if enableHTTP {
		nMethods++
	}
	if enableRTSP {
		nMethods++
	}
	results := make(chan camera.CameraBrands, nMethods)

	tryHttp := func() (camera.CameraBrands, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		u := "http://" + host
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return camera.CameraBrandUnknown, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return camera.CameraBrandUnknown, err
		}
		defer resp.Body.Close()
		bodyB, err := io.ReadAll(resp.Body)
		if err != nil {
			return camera.CameraBrandUnknown, err
		}
		body := string(bodyB)
		return camera.IdentifyCameraFromHTTP(resp.Header, body), nil
	}
	tryRTSP := func() (camera.CameraBrands, error) {
		cameraURL := "rtsp://" + host + ":554"
		url, err := base.ParseURL(cameraURL)
		if err != nil {
			return camera.CameraBrandUnknown, err
		}
		client := &gortsplib.Client{}
		if err := client.Start(url.Scheme, url.Host); err != nil {
			return camera.CameraBrandUnknown, err
		}
		defer client.Close()
		if _, err := client.Options(url); err != nil {
			return camera.CameraBrandUnknown, err
		} else {
			// At least for Hikvision cameras, I can't get any identifying information from the OPTIONS response.
			//fmt.Printf("%v %v\n", resp.StatusCode, resp.StatusMessage)
			//fmt.Printf("%v\n", string(resp.Body))
			//for k, v := range resp.Header {
			//	fmt.Printf("%v: %v\n", k, v)
			//}
			return camera.CameraBrandGenericRTSP, nil
		}
	}

	if enableHTTP {
		go func() {
			model, _ := tryHttp()
			results <- model
		}()
	}
	if enableRTSP {
		go func() {
			model, _ := tryRTSP()
			results <- model
		}()
	}

	// Higher numbers mean a more specific camera result
	cameraSpecificity := func(c camera.CameraBrands) int {
		switch c {
		case camera.CameraBrandUnknown:
			return 0
		case camera.CameraBrandGenericRTSP:
			return 1
		default:
			return 2
		}
	}

	best := camera.CameraBrandUnknown
	for i := 0; i < nMethods; i++ {
		result := <-results
		if cameraSpecificity(result) > cameraSpecificity(best) {
			best = result
		}
		// If we get a specific brand result, return immediately
		if cameraSpecificity(result) >= 2 {
			return result, nil
		}
	}

	return best, nil
}
