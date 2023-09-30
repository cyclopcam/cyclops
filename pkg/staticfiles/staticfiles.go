package staticfiles

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cyclopcam/cyclops/pkg/log"
	"github.com/cyclopcam/cyclops/pkg/www"
)

var reWebpackAsset *regexp.Regexp

// CachedStaticFileServer gzips static files so that we don't pay that compression price
// for every request. Nginx can do this transparently, and we do use that functionality for
// API requests. Nginx also has a proxy cache that can supposedly be used for gzipped
// content, but in my experiments I was unable to get nginx to cache the gzipped content.
// I presume that Nginx is only built to cache gzipped content from files on disk, not from
// files that come from a proxy.
type CachedStaticFileServer struct {
	fsys           fs.FS
	fsRootDir      string // eg "www"
	log            log.Log
	compressLevel  int
	verbose        bool
	apiRoutes      []string         // Any path that begins with an item from apiRoutes produces a 404
	indexIntercept http.HandlerFunc // optional callback during index.html serving (creating for auth hotlink functionality)
	modTime        time.Time        // When we embed files, we Stat() returns a zero time, so we need an alternative

	immutableFilesystem bool // If true, then assume that static files never change (true when running a Docker image)

	compressExtensions map[string]bool // Compress filenames with these extensions

	filesLock sync.Mutex
	files     map[string]*cachedStaticFile // key is absolute filename
}

// cachedStaticFile is an in-memory compressed file
type cachedStaticFile struct {
	Ready        int32 // Updated atomically, once file is ready to be served
	LastModified time.Time
	Path         string
	Compressed   []byte
	Error        error // If there was an error compressing the file, then this is it
}

// absRoot is the root content path.
// apiRoutes are special routes such as /api, which should not serve up your index.html,
// but return a 404 instead.
// The assumption is that your SPA's router module figures out which page to show based
// on the URL, but from the server's perspective, everything except for apiRoutes serves
// up index.html
// indexIntercept can be used to modify a request/response to index.html.
func NewCachedStaticFileServer(fsys fs.FS, fsRootDir string, apiRoutes []string, log log.Log, immutableFilesystem bool, indexIntercept http.HandlerFunc) (*CachedStaticFileServer, error) {
	extensions := map[string]bool{
		"css":  true,
		"js":   true,
		"wasm": true,
		"html": true,
		"svg":  true,
		"map":  true,
		"md":   true,
	}

	// Default to the current time. This is the most conservative thing to do.
	modTime := time.Now()
	if ownPath, err := os.Executable(); err == nil {
		if self, err := os.Stat(ownPath); err == nil {
			// Use modtime of our own executable as the Last Modified time of all embedded files
			modTime = self.ModTime()
		}
	}

	// chunk-vendors.js compressLevel size   time
	//                  9             100665 110ms
	//                  5             101379 10ms
	//
	// From the above numbers, it's not worth it raising the compression level to 9.

	return &CachedStaticFileServer{
		fsys:                fsys,
		fsRootDir:           fsRootDir,
		apiRoutes:           apiRoutes,
		log:                 log,
		verbose:             false,
		compressLevel:       5,
		immutableFilesystem: immutableFilesystem,
		compressExtensions:  extensions,
		files:               map[string]*cachedStaticFile{},
		indexIntercept:      indexIntercept,
		modTime:             modTime,
	}, nil
}

func globRecursive(fsys fs.FS, root string) ([]string, error) {
	files := []string{}

	// The +1 is to remove the leading slash (which is applicable on embedded files)
	chopPrefix := len(root) + 1

	if root == "." || root == "" {
		// This path is for physical filesystem (not embedded)
		root = "." // WalkDir needs a non-empty root
		chopPrefix = 0
	}

	err := fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if d == nil && err != nil {
			// root scan failed
			return err
		}
		if !d.IsDir() {
			files = append(files, path[chopPrefix:])
		}
		return nil
	})
	return files, err
}

func (s *CachedStaticFileServer) ServeFile(w http.ResponseWriter, r *http.Request, relPath string, maxAgeSeconds int) {
	// Prevent FS traversals (eg user requesting example.com/icons/../../../../etc/ssl.key)
	if strings.Contains(relPath, "..") {
		w.WriteHeader(404)
		return
	}

	readerCanGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
	isCompressible := s.isCompressible(relPath) && readerCanGzip
	var cachedFile *cachedStaticFile

	// If immutable, then we can check the cache first
	// This is the expected 99.999% code path for compressed files, when running in production
	if isCompressible && s.immutableFilesystem {
		s.filesLock.Lock()
		cachedFile = s.files[relPath]
		busyOrDone := cachedFile != nil
		if !busyOrDone {
			// We are the first thread to want this, so it is our job to produce the compressed file
			cachedFile = &cachedStaticFile{}
			s.files[relPath] = cachedFile
		}
		s.filesLock.Unlock()
		if busyOrDone {
			s.serveCachedFile(w, r, cachedFile, maxAgeSeconds)
			return
		}
	}

	if s.fsRootDir == "" && strings.HasPrefix(relPath, "/") {
		// Chop off leading slash for the case where our root directory is the root of the filesystem.
		// This path is hit when serving up hot reloadable content from a real file system (not embedded file).
		relPath = relPath[1:]
	}

	file, err := s.fsys.Open(s.fsRootDir + relPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	modTime := stat.ModTime()
	if modTime.IsZero() {
		// This path is always hit for embedded files
		modTime = s.modTime
	}
	if stat.IsDir() {
		w.WriteHeader(404)
		return
	}

	// Serve uncompressed file
	if !isCompressible {
		cacheControl := fmt.Sprintf("max-age=%v, must-revalidate", maxAgeSeconds)
		if www.IsNotModifiedEx(w, r, modTime, cacheControl) {
			if s.verbose {
				s.log.Infof("Serving uncompressed file %v (304 Not Modified)", relPath)
			}
			return
		}
		if s.verbose {
			s.log.Infof("Serving uncompressed file %v", relPath)
		}
		w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(relPath)))
		w.Header().Set("Content-Length", fmt.Sprintf("%v", stat.Size()))
		io.Copy(w, file)
		return
	}

	// There are two code paths that can reach this point:
	// 1. immutableFilesystem is true, and it is our job to compress the file
	// 2. immutableFilesystem is false, and we need to check if the cached file is valid, and proceed down that path

	if !s.immutableFilesystem {
		// This is similar logic to the caching block at the top of the file, but we need to be doing this down here,
		// because we now have the LastModified time of the file on disk.
		s.filesLock.Lock()
		cachedFile = s.files[relPath]
		createNew := false
		if cachedFile != nil &&
			atomic.LoadInt32(&cachedFile.Ready) == 1 &&
			modTime.After(cachedFile.LastModified) {
			// The file on disk has been modified, so discard the cached file, and create a new one.
			// Note that we could also end up with this case:
			//   cachedFile != nil && atomic.LoadInt32(&cachedFile.Ready) == 0 && modTime.After(cachedFile.LastModified)
			// which means that the file was modified after compression started, but compression is not done yet.
			// This doesn't matter, because sooner or later, subsequent threads will notice the staleness.
			if s.verbose {
				s.log.Infof("%v had been modified since compression", relPath)
			}
			createNew = true
		} else if cachedFile == nil {
			createNew = true
		}

		if createNew {
			cachedFile = &cachedStaticFile{}
			s.files[relPath] = cachedFile
		}
		s.filesLock.Unlock()
		if !createNew {
			s.serveCachedFile(w, r, cachedFile, maxAgeSeconds)
			return
		}
	}

	// Compress and store
	start := time.Now()
	cwriter := bytes.Buffer{}
	writer, err := gzip.NewWriterLevel(&cwriter, s.compressLevel)
	if err == nil {
		_, err = io.Copy(writer, file)
		if err == nil {
			err = writer.Flush()
		}
	}
	cachedFile.Error = err
	cachedFile.Path = relPath
	cachedFile.Compressed = cwriter.Bytes()
	cachedFile.Compressed = append([]byte(nil), cachedFile.Compressed...) // trim excess capacity
	cachedFile.LastModified = modTime
	atomic.StoreInt32(&cachedFile.Ready, 1)

	if s.verbose {
		s.log.Infof("Compressing %v took %v ms", relPath, time.Now().Sub(start).Milliseconds())
	}

	s.serveCachedFile(w, r, cachedFile, maxAgeSeconds)
}

func (s *CachedStaticFileServer) isCompressible(filename string) bool {
	ext := path.Ext(filename)
	if len(ext) == 0 {
		return false
	}
	return s.compressExtensions[strings.ToLower(ext[1:])]
}

func (s *CachedStaticFileServer) serveCachedFile(w http.ResponseWriter, r *http.Request, cachedFile *cachedStaticFile, maxAgeSeconds int) {
	// Wait until the responsible thread has finished compressing the file
	for atomic.LoadInt32(&cachedFile.Ready) == 0 {
		time.Sleep(5 * time.Millisecond)
	}

	if cachedFile.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(cachedFile.Error.Error()))
		return
	}

	cacheControl := fmt.Sprintf("max-age=%v, must-revalidate", maxAgeSeconds)

	if www.IsNotModifiedEx(w, r, cachedFile.LastModified, cacheControl) {
		if s.verbose {
			s.log.Infof("Serving cached compressed file %v (304 Not Modified)", cachedFile.Path)
		}
		return
	}

	if s.verbose {
		s.log.Infof("Serving cached compressed file %v", cachedFile.Path)
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(path.Ext(cachedFile.Path)))
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Length", fmt.Sprintf("%v", len(cachedFile.Compressed)))
	io.Copy(w, bytes.NewReader(cachedFile.Compressed))
}

func (s *CachedStaticFileServer) fileExists(file string) bool {
	// remove the leading slash, because os.DirFS() doesn't like it
	if strings.HasPrefix(file, "/") {
		file = file[1:]
	}

	if s.fsRootDir != "" {
		file = path.Join(s.fsRootDir, file)
	}

	st, err := fs.Stat(s.fsys, file)
	return err == nil && !st.IsDir()
}

// This is our static files handler, which gets hit if none of our other routes match.
// Most routes match API entrypoints.
func (s *CachedStaticFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	//s.parent.Log.Infof("Static file request (path=%v), If-Modified-Since='%v'", path, r.Header.Get("If-Modified-Since"))

	maxAgeSeconds := 5

	for _, api := range s.apiRoutes {
		if strings.HasPrefix(path, api) {
			http.Error(w, fmt.Sprintf("The url path '%v' is not a valid API", path), 404)
			return
		}
	}

	// If it's not a genuine file, then it must be index.html
	isIndex := !s.fileExists(path)
	if isIndex {
		if s.indexIntercept != nil {
			s.indexIntercept(w, r)
		}
		s.ServeFile(w, r, "/index.html", maxAgeSeconds)
		return
	}

	// Serve up all other assets (svg, css, etc)
	if reWebpackAsset.MatchString(path) {
		// Although in theory one should be able to set a much longer expiry time, because these
		// assets incorporate a hash, we stick to one day just in case, because screwups in this space DO OCCUR.
		maxAgeSeconds = 86400
	}
	s.ServeFile(w, r, path, maxAgeSeconds)
}

func init() {
	// We use a regex to tell if a URL refers to a file that was built by Webpack.
	// These files incorporate a hash of themselves into their name, so it's safe
	// to cache them for long.
	// If a file does not have a hash in it's name, then we can't issue a very
	// long cache expiry header

	// Positive examples:
	// about.52e3024d.js
	// about.52e3024d.js.map
	// app.b8630bdd.js
	// app.b8630bdd.js.map
	// chunk-vendors.9c15f784.js
	// chunk-vendors.9c15f784.js.map
	// unittest.ad6c7e87.js
	// unittest.ad6c7e87.js.map

	// Negative examples:
	// favicon.ico
	// index.css
	// index.html

	// See TestStaticFileRegex() for more

	reWebpackAsset = regexp.MustCompile(`[^\.]+\.[0-9a-f]{7,}\.[(js)(css)]`)
}
