package www

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/bmharper/cyclops/server/log"
)

// HTTPError is an object that can be panic'ed, and the outer HTTP handler function.
// Will return the appropriate HTTP error message.
type HTTPError struct {
	Code    int
	Message string
}

func (e HTTPError) Error() string {
	return fmt.Sprintf("%v %v", e.Code, e.Message)
}

func Error(code int, message string) HTTPError {
	return HTTPError{code, message}
}

// Panic creates an HTTPError object and panics it.
func Panic(code int, message string) {
	panic(HTTPError{code, message})
}

// PanicBadRequest panics with a 400 Bad Request.
func PanicBadRequest() {
	panic(BadRequest())
}

func BadRequest() HTTPError {
	return HTTPError{http.StatusBadRequest, "Bad Request"}
}

// PanicBadRequestf panics with a 400 Bad Request.
func PanicBadRequestf(format string, args ...interface{}) {
	panic(BadRequestf(format, args...))
}

func BadRequestf(format string, args ...interface{}) HTTPError {
	return HTTPError{http.StatusBadRequest, fmt.Sprintf(format, args...)}
}

// PanicForbidden panics with a 403 Forbidden.
func PanicForbidden() {
	panic(Forbidden())
}

func Forbidden() HTTPError {
	return HTTPError{http.StatusForbidden, "Forbidden"}
}

// PanicNotFound panics with a 404 Not Found.
func PanicNotFound() {
	panic(NotFound())
}

func NotFound() HTTPError {
	return HTTPError{http.StatusNotFound, "Not Found"}
}

// PanicNoContent panics with a 204 No Content.
func PanicNoContent() {
	panic(NoContent())
}

func NoContent() HTTPError {
	return HTTPError{http.StatusNoContent, "No Content"}
}

// PanicServerError panics with a 500 Internal Server Error
func PanicServerError(msg string) {
	panic(ServerError(msg))
}

func ServerError(msg string) HTTPError {
	return HTTPError{http.StatusInternalServerError, msg}
}

// PanicServerErrorf panics with a 500 Internal Server Error
func PanicServerErrorf(format string, args ...interface{}) {
	panic(ServerErrorf(format, args...))
}

func ServerErrorf(format string, args ...interface{}) HTTPError {
	return HTTPError{http.StatusInternalServerError, fmt.Sprintf(format, args...)}
}

// Check causes a panic if err is not nil.
func Check(err error) {
	if err != nil {
		panic(err)
	}
}

// CheckLogged writes the error to the log, and then causes a panic, if err is not nil.
func CheckLogged(l log.Log, err error) {
	if err != nil {
		if l != nil {
			l.Errorf("CheckLogged: %v", err)
		}
		panic(err)
	}
}

// Returns the named form value (typically query value), or panics if the item is empty or missing
func RequiredFormValue(r *http.Request, key string) string {
	v := r.FormValue(key)
	if v == "" {
		PanicBadRequestf("Must specify %v", key)
	}
	return v
}

// Returns the named form value (typically query value) as an int64, or panics if the item is empty, missing, or not parseable as an integer
func RequiredFormInt64(r *http.Request, key string) int64 {
	v := RequiredFormValue(r, key)
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		PanicBadRequestf("Must specify an integer for %v", key)
	}
	return i
}

// Returns the named form value (typically query value) as an int, or panics if the item is empty, missing, or not parseable as an integer
func RequiredFormInt(r *http.Request, key string) int {
	return int(RequiredFormInt64(r, key))
}

// Returns the named form value (typically query value) as an int64, or zero if the item is missing or not parseable as an integer
func FormInt64(r *http.Request, key string) int64 {
	i, _ := strconv.ParseInt(r.FormValue(key), 10, 64)
	return i
}

// Returns the named form value (typically query value) as an int, or zero if the item is missing or not parseable as an integer
func FormInt(r *http.Request, key string) int {
	return int(FormInt64(r, key))
}

// FailedRequestSummary returns a string that you can emit into a log message, when an HTTP that you've made fails
func FailedRequestSummary(resp *http.Response, err error) string {
	return FailedRequestSummaryEx(resp, err, 100)
}

// FailedRequestSummaryEx returns a string that you can emit into a log message, when an HTTP that you've made fails
func FailedRequestSummaryEx(resp *http.Response, err error, maxBodyLen int) string {
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err.Error()
	}
	txt := resp.Status
	if resp.Body != nil {
		all, _ := io.ReadAll(resp.Body)
		allStr := string(all)
		txt += "; "
		if len(allStr) > maxBodyLen {
			txt += allStr[:maxBodyLen] + "..."
		} else {
			txt += allStr
		}
		if txt[len(txt)-1] == '\n' {
			txt = txt[:len(txt)-1]
		}
	}
	return txt
}
