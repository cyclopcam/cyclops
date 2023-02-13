# Install

_Only tested on Ubuntu 22.04_

    sudo apt install libavformat-dev libswscale-dev ffmpeg cmake gcc g++ pkg-config libturbojpeg0-dev wireguard wireguard-tools

## Dev environment

-   Install Go
-   Install the apt packages mentioned above
-   Build ncnn (make sure you have done `git submodule update --init`)
    -   `cd ncnn`
    -   `mkdir build`
    -   `cd build`
    -   `cmake -DNCNN_SIMPLEOCV=1 ..`
    -   `make -j` (if you have sufficient RAM ...)
    -   `make -j2` (... for a Rpi4 with 4GB RAM)
-   Build Simd (make sure you have done `git submodule update --init`)
    -   `cd Simd`
    -   `mkdir build`
    -   `cd build`
    -   `cmake ../prj/cmake`
    -   `make -j` (if you have sufficient RAM ...)
    -   `make -j2` (... for a Rpi4 with 4GB RAM)
-   Install nvm
-   Use `nvm install v18.2.0` (The version is possibly not important, but this
    is the exact version I used when creating this document)
-   In `www`, do `npm install`, then `npm run build`
-   In `appui`, do `npm install`, then `npm run build`
-   `go build -o bin/kernelwg cmd/kernelwg/*.go`
-   First run of `go run cmd/cyclops/cyclops.go` takes a few minutes on RPi4,
    mostly due to single-threaded build of `github.com/mattn/go-sqlite3`

Once setup, you should be able to run the server and the interface:

-   `sudo bin/kernelwg`
-   `go run cmd/cyclops/cyclops.go`
-   `npm run dev -- --host` (from the `www` directory). The `-- --host` allows
    you to connect from external devices.
-   `npm run dev -- --host` (from the `appui` directory, for working on the
    native app overlay)

### WSL / Using a VM for Dev

If you're using WSL, or any kind of VM for dev work, then you're probably behind
a NAT of that VM. This will break the camera LAN scanning mechanism, because
Cyclops will scan the VM's NAT network, instead of your actual home network. To
work around this, you can specify your LAN network with the `--ip` flag. For
example, to launch the server, use this script
`scripts/server --ip 192.168.1.10`, if your own IP is `192.168.1.10`, and the
cameras on your LAN are on the `192.168.1.x` network.

### Accessing the server API

Although BASIC authentication is very useful for debugging, it is an inherently
slow authentication mechanism, and because of this we disable it for most APIs.

In order to debug the server API from curl or similar tools, the easiest method
is to acquire a bearer token, and use that token for subsequent requests.

Example:

```sh
$ curl -X POST -u USERNAME:PASSWORD http://localhost:8080/api/auth/login?loginMode=BearerToken
$ {"bearerToken":"h1cPWbUyCKBeEPc8NgW8Fj4q+TpgRUIuvezTr0NFV80="}
$ curl -H "Authorization: Bearer h1cPWbUyCKBeEPc8NgW8Fj4q+TpgRUIuvezTr0NFV80=" -o training.zip http://localhost:8080/api/train/getDataset
```

# Other Topics

### Seeking through videos

When labelling a video, we very much want seeking through the video to be fast
and smooth. This is handled for us without any effort on desktop Chrome, where
it will seek immediately to whatever timepoint you ask for, simply by setting
video.currentTime. However, on mobile, this doesn't work. I can't tell for sure
what's happening, but it seems to seek only to keyframes on mobile. In addition,
it seems to debounce the seek event, if the distance between keyframes is 10
frames (which is what my cameras are set at). Some of this behaviour may be due
to the way that I'm encoding the mp4 files (i.e. direct from camera, without any
transcoding). However, I have tried using ffmpeg to re-encode one of these files
with "-g 10", and I still get the bad mobile seeking behaviour with that, so
that's what leads me to believe that this really does have a tight dependence on
keyframe interval.

The first thing I tried was a dead simple codec that just computes the signed
difference between successive frames, and compresses them with lz4. This
performs terribly. A compressed frame is only about 50% the size of a raw RGBA
frame.

After that, I experimented with simply re-encoding the video using ffmpeg, but
using the "-g" parameter to specify the maximum keyframe interval. What seems to
produce reasonable results is "-g 5 -crf 25". In my test video, this makes it
about twice it's original size.

Note that if WebCodecs became widely available (notably on common Android
WebView releases, and Safari), then we'd be able to use that to decode all
frames of a short clip into RAM, and seek trivially wherever we like. But until
such time, we need to generate a keyframe-heavy clip.

Ony my Xiaomi Note 9 Pro, I get annoying jitter when seeking backwards, even
with a "-g 3" video. However, on a Xiaomi Note 10 Pro, that behaviour is already
gone, so there's hope that eventually the mobile browsers will match the desktop
browsers smooth random seeking, and we don't need to do anything else. This may
not be a Chrome logic thing, but a performance thing (e.g. is the phone's CPU
capable of decoding fast enough to make the experience interactive).

# Architecture

(These are a bench of self notes - reminders of topics to cover in comprehensive
docs)

-   Low Res, High Res streams
-   Permanent Storage
-   Recent Event Storage
-   Where it's OK to panic() in Go code (basically, you may only using panic as
    a flow control mechanism if the function name starts with `http`. Otherwise,
    you must use an `error` return)
-   Clarify startup order - eg when is it OK for the background recorder to
    assume that cameras are live. If a camera is in Server, does that mean it's
    live? Or how do we check for liveness.. how do we schedule an action that
    will start once the camera becomes live?
