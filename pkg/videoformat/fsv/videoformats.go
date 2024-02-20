package fsv

// A video file format must support the VideoFormat interface in order to be
// used by fsv.
type VideoFormat interface {
	IsVideoFile(filename string) bool
}
