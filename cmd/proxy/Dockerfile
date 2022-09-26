# docker build -t cyclops-proxy -f cmd/proxy/Dockerfile .

FROM golang:1.19 as build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN go build -o /out/proxy cmd/proxy/proxy.go

FROM ubuntu:22.04

COPY --from=build /out/proxy /proxy
CMD ["/proxy"]
