package buildinfo

// Multiarch is filled in by the Debian build system.
// It's the directory you see in /usr/lib/XXX, such as /usr/lib/x86_64-linux-gnu, or /usr/lib/aarch64-linux-gnu.
// Inside this, we store our plugins, with a full path such as: /usr/lib/aarch64-linux-gnu/cyclops/libcyclopshailo.so
// If the value of Multiarch is "unknown", then we ignore this path.
var Multiarch = "unknown"