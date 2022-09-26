# Proxy

This is a proxy that allows mobile apps to talk to Cyclops servers running behind a NAT (eg inside a home).

Servers add themselves to the proxy by issuing a call to /api/register. The only input is their
Wireguard public key. We assign that server an IP in the 10.0.0.0 subnet, and return the IP
to the caller. The caller now knows to configure it's own Wireguard setup with that IP address.

## Root Access

Controlling Wireguard requires root priviledges. In order to limit our attack surface, we do not run
the proxy server as root. Instead, we run a companion program who's only job is to talk to Wireguard.
This companion program lives in cmd/kernelwg. We use Go's GOB encoder to marshal data between
proxy and kernelwg.
