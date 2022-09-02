# Install

sudo apt install libavformat-dev libswscale-dev gcc pkg-config libturbojpeg0-dev

## dev env
* Install Go
* Install the apt packages mentioned above
* Install nvm
* Use `nvm install v18.2.0` (The version is possibly not important, but this is the exact version I used when creating this document)
* In `www`, do `npm install`
* First run of `go run cyclops.go` takes a few minutes on RPi4, mostly due to single-threaded build of `github.com/mattn/go-sqlite3`

Once setup, you should be able to run the server and the interface:
* `go run cyclops.go`
* `npm run dev -- --host` (from the `www` directory). The `-- --host` allows you to connect from external devices.

### Seeking through videos
When labelling a video, we very much want seeking through the video to be fast and smooth. This is handled for us
without any effort on desktop Chrome, where it will seek immediately to whatever timepoint you ask for, simply by
setting video.currentTime. However, on mobile, this doesn't work. I can't tell for sure what's happening, but it
seems to seek only to keyframes on mobile. In addition, it seems to debounce the seek event, if the distance between
keyframes is 10 frames (which is what my cameras are set at). Some of this behaviour may be due to the way that 
I'm encoding the mp4 files (i.e. direct from camera, without any transcoding). However, I have tried using ffmpeg
to re-encode one of these files with "-g 10", and I still get the bad mobile seeking behaviour with that, so that's
what leads me to believe that this really does have a tight dependence on keyframe interval.

The first thing I tried was a dead simple codec that just computes the signed difference between successive frames,
and compresses them with lz4. This performs terribly. A compressed frame is only about 50% the size of a raw RGBA
frame. 

After that, I experimented with simply re-encoding the video using ffmpeg, but using the "-g" parameter to specify
the maximum keyframe interval. What seems to produce reasonable results is "-g 5 -crf 25". In my test video, this
makes it about twice it's original size.

Note that if WebCodecs became widely available (notably on common Android WebView releases, and Safari), then
we'd be able to use that to decode all frames of a short clip into RAM, and seek trivially wherever we like.
But until such time, we need to generate a keyframe-heavy clip.

Ony my Xiaomi Note 9 Pro, I get annoying jitter when seeking backwards, even with a "-g 3" video. However, on
a Xiaomi Note 10 Pro, that behaviour is already gone, so there's hope that eventually the mobile browsers will
match the desktop browsers smooth random seeking, and we don't need to do anything else.

# Architecture

(These are a bench of self notes - reminders of topics to cover in comprehensive docs)

* Low Res, High Res streams
* Permanent Storage
* Recent Event Storage
* Where it's OK to panic()
* Clarify startup order - eg when is it OK for the background recorder to assume that cameras are live.
  If a camera is in Server, does that mean it's live? Or how do we check for liveness.. how do we schedule
  an action that will start once the camera becomes live?