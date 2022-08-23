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

# Architecture

(These are a bench of self notes - reminders of topics to cover in comprehensive docs)

* Low Res, High Res streams
* Permanent Storage
* Recent Event Storage
* Where it's OK to panic()
* Clarify startup order - eg when is it OK for the background recorder to assume that cameras are live.
  If a camera is in Server, does that mean it's live? Or how do we check for liveness.. how do we schedule
  an action that will start once the camera becomes live?