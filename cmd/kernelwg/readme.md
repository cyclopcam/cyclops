# kernel wireguard

`kernelwg` is a tiny program that runs as root. It's sole job is to alter WireGuard state.
Changing WireGuard state and IP routes requires root priviledges. Instead of running our entire
proxy service as root, we only run this portion as root, and we communicate over a socket.

I don't really know what I'm doing here. I suspect I ought to be looking into tools like
https://justine.lol/pledge/.