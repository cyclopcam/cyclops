package www

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/julienschmidt/httprouter"
)

// RunProtected runs 'func' inside a panic handler that recognizes our special errors,
// and sends the appropriate HTTP response if a panic does occur.
func RunProtected(log log.Log, w http.ResponseWriter, r *http.Request, handler func()) {
	defer func() {
		if rec := recover(); rec != nil {
			if hErr, ok := rec.(HTTPError); ok {
				log.Infof("Failed request %v: %v %v", r.URL.Path, hErr.Code, hErr.Message)
				SendError(w, hErr.Message, hErr.Code)
			} else if hErr, ok := rec.(*HTTPError); ok {
				log.Infof("Failed request %v: %v %v", r.URL.Path, hErr.Code, hErr.Message)
				SendError(w, hErr.Message, hErr.Code)
			} else if err, ok := rec.(runtime.Error); ok {
				// Show stack trace on runtime error
				log.Errorf("Runtime panic error %v: %v", r.URL.Path, err)
				log.Errorf("Stack Trace: %v", string(debug.Stack()))
				SendError(w, err.Error(), http.StatusInternalServerError)
			} else if err, ok := rec.(error); ok {
				// No stack trace on generic error
				log.Errorf("Panic error %v: %v", r.URL.Path, err)
				//log.Errorf("Stack Trace: %v", string(debug.Stack()))
				SendError(w, err.Error(), http.StatusInternalServerError)
			} else if err, ok := rec.(string); ok {
				log.Errorf("Panic string %v: %v", r.URL.Path, err)
				SendError(w, err, http.StatusInternalServerError)
			} else {
				log.Errorf("Unrecognized panic %v: %v", r.URL.Path, rec)
				SendError(w, "Unrecognized panic", http.StatusInternalServerError)
			}
		}
	}()

	handler()
}

// Handle adds a protected HTTP route to router (ie handle will run inside RunProtected, so you get a panic handler).
func Handle(log log.Log, router *httprouter.Router, method, path string, handle httprouter.Handle) {
	wrapper := func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		RunProtected(log, w, r, func() { handle(w, r, p) })
	}
	router.Handle(method, path, wrapper)
}

// ParseID parses a 64-bit integer, and returns zero on failure.
func ParseID(s string) int64 {
	id, _ := strconv.ParseInt(s, 10, 64)
	return id
}

// Returns (value, true) if a query value exists (an empty string counts as existence).
// Returns ("", false) if the query value does not exist
func QueryValueEx(r *http.Request, s string) (string, bool) {
	val, exists := r.URL.Query()[s]
	if exists {
		if len(val) > 0 {
			return val[0], true
		} else {
			return "", true
		}
	} else {
		return "", false
	}
}

// Returns the named query value (or an empty string)
func QueryValue(r *http.Request, key string) string {
	return r.URL.Query().Get(key)
}

// Returns the named form value (typically query value), or panics if the item is empty or missing
func RequiredQueryValue(r *http.Request, key string) string {
	v := QueryValue(r, key)
	if v == "" {
		PanicBadRequestf("Must specify %v", key)
	}
	return v
}

// Returns the named form value (typically query value) as an int64, or panics if the item is empty, missing, or not parseable as an integer
func RequiredQueryInt64(r *http.Request, key string) int64 {
	v := RequiredQueryValue(r, key)
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		PanicBadRequestf("Must specify an integer for %v", key)
	}
	return i
}

// Returns the named form value (typically query value) as an int, or panics if the item is empty, missing, or not parseable as an integer
func RequiredQueryInt(r *http.Request, key string) int {
	return int(RequiredQueryInt64(r, key))
}

// Returns the named form value (typically query value) as an int64, or zero if the item is missing or not parseable as an integer
func QueryInt64(r *http.Request, key string) int64 {
	i, _ := strconv.ParseInt(r.FormValue(key), 10, 64)
	return i
}

// Returns the named form value (typically query value) as an int, or zero if the item is missing or not parseable as an integer
func QueryInt(r *http.Request, key string) int {
	return int(QueryInt64(r, key))
}

// EncodeQuery returns a key=value&key2=value2 string for a URL
func EncodeQuery(kv map[string]string) string {
	s := ""
	for k, v := range kv {
		s += url.QueryEscape(k) + "=" + url.QueryEscape(v) + "&"
	}
	if len(s) != 0 {
		// remove final ampersand
		s = s[:len(s)-1]
	}
	return s
}

// Read the request body, but limit the number of bytes that will be read, to ensure
// the server isn't loaded heavily by faulty or malicious requests
func ReadLimited(w http.ResponseWriter, r *http.Request, maxBodyBytes int64) []byte {
	if r.Body == nil {
		Panic(http.StatusBadRequest, "ReadLimited failed: Request body is empty")
	}
	defer r.Body.Close()
	reader := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := ioutil.ReadAll(reader)
	Check(err)
	return body
}

// ReadString reads the body of the request, and returns it as a string
func ReadString(w http.ResponseWriter, r *http.Request, maxBodyBytes int64) string {
	b := ReadLimited(w, r, maxBodyBytes)
	return string(b)
}

// ReadJSON reads the body of the request, and unmarshals it into 'obj'.
func ReadJSON(w http.ResponseWriter, r *http.Request, obj interface{}, maxBodyBytes int64) {
	if r.Body == nil {
		Panic(http.StatusBadRequest, "ReadJSON failed: Request body is empty")
	}
	reader := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()
	var err error
	decoder := json.NewDecoder(reader)
	err = decoder.Decode(obj)
	if err != nil {
		Panic(http.StatusBadRequest, "ReadJSON failed: Failed to decode JSON - "+err.Error())
	}
}

// Read a comma-separated list of integer IDs
func ReadIDList(r *http.Request) []int64 {
	if r.Body == nil {
		Panic(http.StatusBadRequest, "ReadIDList failed: Request body is empty")
	}
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		PanicBadRequestf("ReadIDList ReadAll failed: %v", err)
	}
	r.Body.Close()
	return SplitIDList(string(body))
}

// Split a list of IDs such as "345,789" by comma, and return the list as int64s. Will panic on bad input.
func SplitIDList(idList string) []int64 {
	ids := []int64{}
	parts := strings.Split(idList, ",")
	for _, p := range parts {
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			PanicBadRequestf("SplitIDList ParseInt(%v) failed: %v", p, err)
		}
		ids = append(ids, id)
	}
	return ids
}

// Set cache headers which indicate that this resource is immutable
// Apparently chrome is not going to implement the immutable cache-control
// tag, so we just set a long expiry date.
func CacheImmutable(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=2592000, immutable") // this is 30 days
}

// Set cache headers for a given expiry in seconds
func CacheSeconds(w http.ResponseWriter, seconds int) {
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%v", seconds))
}

// Set cache headers instructing the client never to cache
func CacheNever(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=0"))
}

// IsNotModified checks for an If-Modified-Since header, and if modifiedAt is
// before or equal to the header value, then we write a 304 Not Modified to w,
// and return true.
// If, on the other hand, modifiedAt is greater than If-Modified-Since (or
// the If-Modified-Since header is not present), then we set the Last-Modified
// header on w, and return false.
func IsNotModifiedEx(w http.ResponseWriter, r *http.Request, modifiedAt time.Time, cacheControl string) bool {
	// Rounding is a problem here, since modifiedAt will often be stored with higher precision
	// than the HTTP header. The HTTP header has single-second precision.
	// So the first question is:
	// In the conversion from time.Time to HTTP time, does it round to the nearest second, or truncate?
	// The answer: it truncates.
	// So to keep things simple, we also truncate.
	// We even take it a step further, and use our truncated value when creating the Last-Modified header.
	modifiedAt = modifiedAt.UTC().Truncate(time.Second)
	ifModifiedSinceStr := r.Header.Get("If-Modified-Since")
	ifModifiedSince := time.Time{}
	var err error
	if ifModifiedSinceStr != "" {
		ifModifiedSince, err = time.Parse(time.RFC1123, ifModifiedSinceStr)
	}
	if err != nil || modifiedAt.After(ifModifiedSince) {
		w.Header().Set("Last-Modified", modifiedAt.Format(http.TimeFormat))
		w.Header().Set("Cache-Control", cacheControl)
		return false
	}
	w.WriteHeader(http.StatusNotModified)
	return true
}

func IsNotModified(w http.ResponseWriter, r *http.Request, modifiedAt time.Time) bool {
	return IsNotModifiedEx(w, r, modifiedAt, "max-age=0, must-revalidate")
}

// SendError is identical to the standard library http.Error(), except that we don't append a \n to the message body
func SendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	w.Write([]byte(message))
}

// SendJSON encodes 'obj' to JSON, and sends it as an HTTP application/json response.
func SendJSON(w http.ResponseWriter, obj interface{}) {
	SendJSONOpt(w, obj, false)
}

func SendJSONOpt(w http.ResponseWriter, obj interface{}, pretty bool) {
	// TODO: compress
	w.Header().Set("Content-Type", "application/json")
	var b []byte
	var err error
	if pretty {
		b, err = json.MarshalIndent(obj, "", "\t")
	} else {
		b, err = json.Marshal(obj)
	}
	Check(err)
	w.Write(b)
}

func SendJSONRaw(w http.ResponseWriter, raw string) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(raw))
}

// SendText sends text as an HTTP text/plain response
func SendText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain")
	b := []byte(text)
	w.Write(b)
}

// SendFmt serializes 'any' via fmt.Sprintf("%v"), and sends it as text/plain
func SendFmt(w http.ResponseWriter, any interface{}) {
	w.Header().Set("Content-Type", "text/plain")
	b := fmt.Sprintf("%v", any)
	w.Write([]byte(b))
}

// SendJSONID sends the ID as {"id":<value>}
func SendJSONID(w http.ResponseWriter, id int64) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(fmt.Sprintf(`{"id":%v}`, id)))
}

// SendJSONBool sends "true" or "false" as an application/json response
func SendJSONBool(w http.ResponseWriter, v bool) {
	w.Header().Set("Content-Type", "application/json")
	if v {
		w.Write([]byte("true"))
	} else {
		w.Write([]byte("false"))
	}
}

// SendID sends the ID as text/plain
func SendID(w http.ResponseWriter, id int64) {
	SendInt64(w, id)
}

// SendInt64 sends the number as text/plain
func SendInt64(w http.ResponseWriter, id int64) {
	w.Header().Set("Content-Type", "text/plain")
	b := fmt.Sprintf("%v", id)
	w.Write([]byte(b))
}

// SendOK sends "OK" as a text/plain response.
func SendOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("OK"))
}

// SendFileDownload sends a file for download
func SendFileDownload(w http.ResponseWriter, filename, contentType string, content []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%v"`, filename))
	w.Write(content)
}

// SendFile sends a file (as direct content, not download)
func SendFile(w http.ResponseWriter, r *http.Request, filename, contentType string) {
	// http.ServeFile implements ranges, which is critical for some features, eg <video> playback
	http.ServeFile(w, r, filename)
}

// SendTempFile calls SendFile, and then deletes the file when finished
func SendTempFile(w http.ResponseWriter, r *http.Request, filename, contentType string) {
	SendFile(w, r, filename, contentType)
	os.Remove(filename)
}
