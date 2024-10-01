package requests

// requests is a library for making JSON requests to HTTP APIs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func RequestJSON[T any](method, url string, body any) (response *T, err error) {
	bodyB, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(bodyB))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("%v. %v", resp.Status, string(msg))
	}
	var responseObj T
	if err := json.NewDecoder(resp.Body).Decode(&responseObj); err != nil {
		return nil, fmt.Errorf("%v. %w", resp.Status, err)
	}
	response = &responseObj
	return
}
