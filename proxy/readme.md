# Proxy

This is a proxy that allows mobile apps to talk to Cyclops servers running
behind a NAT (eg inside a home).

Servers add themselves to the proxy by issuing a call to /api/register. The only
input is their Wireguard public key. We assign that server an IP in the 10.0.0.0
subnet, and return the IP to the caller. The caller now knows to configure its
own Wireguard setup with that IP address.

## Root Access

Controlling Wireguard requires root privileges. In order to limit our attack
surface, we use setuid + setgid to lower our privileges. However, before doing
that, we launch a copy of ourselves as root. That root process is the one that
performs the Wireguard manipulations. The two processes communicate via a unix
domain socket. We use Go's GOB encoder to marshal data between them.

## Dev Env

> scripts/proxy/compose

> go build -o bin/cyclopsproxy && sudo bin/cyclopsproxy

You should now be able to hit the proxy, eg

> curl
> localhost:8082/proxy/w8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t_g4eI=/api/ping

## Server Setup

1. Install Docker
2. Run Postgres with Docker
   `docker run -d --restart unless-stopped -v /deploy/proxydb:/var/lib/postgresql/data -p 127.0.0.1:5432:5432 -e POSTGRES_PASSWORD=....... postgres:14.5-alpine -c 'listen_addresses=*'`
3. Install caddy. I use caddy instead of nginx, because nginx needs dedicated
   routes in order to forward websockets.
   [caddyserver.com/docs/install](https://caddyserver.com/docs/install#debian-ubuntu-raspbian)
4. Grant caddy ability to listen on low ports:
   `sudo setcap cap_net_bind_service=+ep $(which caddy)`

Create a "cyclopsproxy" user for the proxy service

```
sudo groupadd --system cyclopsproxy

sudo useradd --system \
    --gid cyclopsproxy \
    --create-home \
    --home-dir /var/lib/cyclopsproxy \
    --shell /usr/sbin/nologin \
    --comment "Cyclops proxy server" \
    cyclopsproxy
```

```
caddy reverse-proxy --from proxy-cpt.cyclopcam.org --to localhost:8082
```

```
psql -h localhost -U postgres
```

```
go build -o bin/cyclopsproxy cmd/proxy/proxy.go
scp bin/cyclopsproxy ubuntu@proxy-cpt.cyclopcam.org:~/
ssh ubuntu@proxy-cpt.cyclopcam.org "sudo systemctl stop cyclopsproxy && sudo cp /home/ubuntu/cyclopsproxy /deploy && sudo systemctl start cyclopsproxy"
```

Use deployment/proxy-services to create systemd services for caddy and
cyclopsproxy.

## TODO

-   Remove peers that haven't spoken for X days (eg 3 days)
-   Populate last_traffic_at with data from Wireguard
-   Require some kind of proof of work before adding a peer (eg Android App
    installation)
-   Figure out a way to tunnel encrypted traffic end-to-end, from the app all
    the way through to the remote server. The best thing I can think of so far
    is this:
    -   Run an HTTP proxy server on the mobile device
    -   Let that proxy do all of its comms over a userspace wireguard channel
    -   UPDATE - The above idea is dead in the water, because you can only run
        one VPN at a time on Android, and I don't want to force users to choose
        between Cyclops and their existing VPN.
