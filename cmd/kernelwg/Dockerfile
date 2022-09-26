# docker build -t cyclops-kernelwg -f cmd/kernelwg/Dockerfile .

FROM golang:1.19 as build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN go build -o /out/kernelwg cmd/kernelwg/kernelwg.go

FROM ubuntu:22.04

COPY --from=build /out/kernelwg /kernelwg
CMD ["/kernelwg"]
