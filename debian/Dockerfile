# Build steps:
# 1. Build the builder image:
#   docker build --tag builder --build-arg BUILDOS=ubuntu:24.04 --build-arg VERSION=v1.0.2 --build-arg BUILDNUMBER=1 -f debian/Dockerfile debian
#   NOTE: The VERSION above is the git revision/tag that is cloned.
# 2. Extract the files:
#   mkdir incoming
#   rm incoming/*
#   docker run --rm -v ./incoming:/mnt/incoming builder
#   sudo chown -R $USER incoming
#   rsync -avz incoming/ ~/dev/other-cyclops/aptbuild/incoming/
# 2. Step 2 as a single line:
#   rm -rf incoming && mkdir -p incoming && docker run --rm -v ./incoming:/mnt/incoming builder && sudo chown -R $USER incoming && rsync -avz incoming/ ~/dev/other-cyclops/aptbuild/incoming/

ARG BUILDOS
FROM ${BUILDOS}

# Install all the dependencies that 'prebuild' installs, so that we have them in a nicely cached layer.

RUN apt-get update
RUN apt-get install -y apt-transport-https ca-certificates curl git libavformat-dev libswscale-dev ffmpeg build-essential cmake devscripts debhelper-compat gcc g++ pkg-config libturbojpeg0-dev wireguard wireguard-tools

# Install Go
ENV GOVERSION=1.24.2
ENV GOARCH=amd64
ENV GOARCHIVE=go$GOVERSION.linux-$GOARCH.tar.gz
RUN curl -o $GOARCHIVE https://dl.google.com/go/$GOARCHIVE
RUN tar -C /usr/local -xzf $GOARCHIVE
ENV PATH=$PATH:/usr/local/go/bin

# Install nvm
RUN curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.3/install.sh | bash \
	&& . ~/.nvm/nvm.sh \
	&& nvm install 18 \
	&& nvm use 18

WORKDIR /build
ARG VERSION
RUN git clone --depth 1 --branch ${VERSION} https://github.com/cyclopcam/cyclops

WORKDIR /build/cyclops
RUN git submodule update --init --recursive

ENV MAKEJ=-j8

WORKDIR /build/cyclops/ncnn
RUN mkdir -p build && cd build && cmake -DNCNN_SIMPLEOCV=1 -DNCNN_BUILD_EXAMPLES=OFF -DNCNN_BUILD_BENCHMARK=OFF -DNCNN_BUILD_TOOLS=OFF .. && make $MAKEJ

WORKDIR /build/cyclops/Simd
RUN mkdir -p build && cd build && cmake -DSIMD_TEST=OFF ../prj/cmake && make $MAKEJ

WORKDIR /build/cyclops/www
RUN . ~/.nvm/nvm.sh && npm install
RUN . ~/.nvm/nvm.sh && npm run build

WORKDIR /build/cyclops/nnaccel/hailo
RUN ./build

# Cyclops will get rebuilt by debian/fullbuild, but that will be fast due to Go's build cache.
WORKDIR /build/cyclops
RUN mkdir -p /build/output
RUN go mod download
RUN go build -o /build/output/cyclops cmd/cyclops/cyclops.go

ARG BUILDNUMBER
ARG REMOTE
WORKDIR /build/cyclops
RUN debian/prebuild ${BUILDNUMBER} dummy
RUN debian/build ${BUILDNUMBER} dummy

#CMD ["bash", "-c", "cp /build/*.deb /mnt/incoming && cp /build/*.changes /mnt/incoming && cp /build/*.buildinfo /mnt/incoming"]
# *.{deb,changes,buildinfo}
CMD ["bash", "-c", "cp /build/*.{deb,changes,buildinfo} /mnt/incoming"]
