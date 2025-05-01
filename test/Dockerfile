# This is used for testing deployment issues that are hard to test on a local dev setup
# For example, does our 'cyclops' user get created properly, and is the home directory
# correct, and does config.sqlite go into the correct place, etc.

# Steps to run:
# go build -o bin/cyclops cmd/cyclops/cyclops.go
# docker build -t cyclops-test -f test/Dockerfile bin
# docker run -it --rm cyclops-test

FROM ubuntu:24.04 AS build

RUN apt-get update
RUN apt-get install -y apt-transport-https ca-certificates curl libavformat-dev libswscale-dev ffmpeg cmake gcc g++ pkg-config libturbojpeg0-dev wireguard wireguard-tools

# Install Go
ENV GOVERSION=1.24.2
ENV GOARCH=amd64
ENV GOARCHIVE=go$GOVERSION.linux-$GOARCH.tar.gz
RUN curl -o $GOARCHIVE https://dl.google.com/go/$GOARCHIVE
RUN tar -C /usr/local -xzf $GOARCHIVE
ENV PATH=$PATH:/usr/local/go/bin

COPY ncnn/ build/ncnn/
COPY Simd/ build/Simd/

ENV MAKEJ=-j8

WORKDIR /build/ncnn
RUN mkdir -p build && cd build && cmake -DNCNN_SIMPLEOCV=1 -DNCNN_BUILD_EXAMPLES=OFF -DNCNN_BUILD_BENCHMARK=OFF -DNCNN_BUILD_TOOLS=OFF .. && make $MAKEJ

WORKDIR /build/Simd
RUN mkdir -p build && cd build && cmake -DSIMD_TEST=OFF ../prj/cmake && make $MAKEJ

# Install nvm
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash \
	&& . ~/.nvm/nvm.sh \
	&& nvm install 18 \
	&& nvm use 18

COPY go.mod go.sum /build/
WORKDIR /build
RUN go mod download

COPY www/ /build/www/
WORKDIR /build/www
RUN . ~/.nvm/nvm.sh && npm install
RUN . ~/.nvm/nvm.sh && npm run build

COPY nnaccel/ /build/nnaccel/
WORKDIR /build/nnaccel/hailo
RUN ./build

COPY cmd/ /build/cmd/
COPY pkg/ /build/pkg/
COPY server/ /build/server/
WORKDIR /build
RUN go build -o /build/cyclops cmd/cyclops/cyclops.go

FROM ubuntu:24.04

#RUN apt-get update && apt-get install -y libavcodec60 libavutil58 libgomp1 libswscale7 libturbojpeg wireguard wireguard-tools \
#	&& rm -rf /var/lib/apt/lists/* \
#	&& apt-get clean

RUN apt-get update && apt-get install -y libavcodec60 libavformat60 libavutil58 libgomp1 libswscale7 libturbojpeg wireguard wireguard-tools

COPY --from=build /build/cyclops /usr/bin/cyclops

ENTRYPOINT [ "/usr/bin/cyclops" ]
#CMD ["/usr/bin/cyclops", "--vpn", "--username", "cyclops", "--config", "/var/lib/cyclops/config.sqlite"]
#CMD ["/usr/bin/cyclops", "--username", "cyclops", "--config", "/var/lib/cyclops/config.sqlite"]