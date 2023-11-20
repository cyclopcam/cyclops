package www

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Perform the request, and if any errors occurs (transport or non-200 status code), return an error
// This does the work for you of checking for a non-200 response, reading the response body,
// and turning it into an error.
func Do(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		respB, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP error %v (%v)", resp.Status, string(respB))
	}
	return resp, nil
}

func FetchJSON(req *http.Request, output any) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respB, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP error %v (%v)", resp.Status, string(respB))
	}
	return json.NewDecoder(resp.Body).Decode(output)
}
