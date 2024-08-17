# wgroot

The code in this package runs in a root-mode process, and is responsible
for manipulating Wireguard state.

There is an accompanying user-mode process that talks to this module
a unix socket.

This design allows us to run the user-mode process in a lower privilege
state.