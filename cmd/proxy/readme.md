To launch dev env:

> scripts/proxy/compose

(as root)
> go run cmd/kernelwg/kernelwg.go

(not as root)
> go run cmd/proxy/proxy.go

You should now be able to hit the proxy, eg
> curl localhost:8082/proxy/w8TFxsfIycrLzM3Oz9DR0tPU1dbX2Nna29zd3t_g4eI=/api/ping