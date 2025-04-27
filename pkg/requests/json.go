package requests

// requests is a library for making JSON requests to HTTP APIs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ResponseError struct {
	StatusCode int
	Message    string
}

func (e ResponseError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func NewError(statusCode int, message string) error {
	return &ResponseError{
		StatusCode: statusCode,
		Message:    message,
	}
}

func RequestJSON[ResponseT any](method, url string, body any) (response *ResponseT, err error) {
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
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(resp.Body)
		return nil, NewError(resp.StatusCode, string(msg))
	}
	var responseObj ResponseT
	if err := json.NewDecoder(resp.Body).Decode(&responseObj); err != nil {
		return nil, fmt.Errorf("%v. %w", resp.Status, err)
	}
	response = &responseObj
	return
}
