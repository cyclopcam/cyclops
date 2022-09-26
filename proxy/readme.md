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

## Dev Env

> scripts/proxy/compose

(as root)
> go run cmd/kernelwg/kernelwg.go

(not as root)
> go run cmd/proxy/proxy.go

You should now be able to hit the proxy, eg
> curl localhost:8082/proxy/w8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t_g4eI=/api/ping

## Server Setup

1. Install Docker
2. Copy 

```
go build -o bin/proxy cmd/proxy/proxy.go
go build -o bin/kernelwg cmd/kernelwg/kernelwg.go
scp bin/proxy ubuntu@13.246.23.60:~/
scp bin/kernelwg ubuntu@13.246.23.60:~/
ssh ubuntu@13.246.23.60 "sudo cp /home/ubuntu/proxy /deploy && sudo cp /home/ubuntu/kernelwg /deploy"
```