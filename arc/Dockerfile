# docker build -t arc -f arc/Dockerfile .

FROM node:18-alpine as www

# /build/server/www is the output directory of "npm run build" (simulating dev environment)
RUN mkdir -p /build/server/www

WORKDIR /build/www
COPY arc/www/package.json arc/www/package-lock.json ./
RUN npm install
COPY arc/www/ ./
RUN npm run build

############################################################
FROM ubuntu:22.04 as build

RUN apt-get update && apt-get install -y libjpeg-turbo8-dev wget build-essential pkg-config libturbojpeg0-dev libavformat-dev libswscale-dev ffmpeg

# Install Go 1.22
RUN wget https://go.dev/dl/go1.21.2.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.21.2.linux-amd64.tar.gz
ENV PATH=$PATH:/usr/local/go/bin

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Wipe the built 'www' from a local dev-environment build
# The 'www' build is baked into the Go service with go:embed
RUN rm -rf arc/server/www
COPY --from=www /build/server/www ./arc/server/www
RUN CGO_ENABLED=1 go build -o /out/arc cmd/arc/arc.go

############################################################
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y ffmpeg libturbojpeg

COPY --from=build /out/arc /arc
ENTRYPOINT ["/arc"]
