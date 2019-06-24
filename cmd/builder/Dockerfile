FROM golang:1.12 AS go-builder

ENV GOPROXY=https://proxy.golang.org
ENV GO111MODULE=on

WORKDIR /project

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -ldflags='-s -w' -o /builder ./cmd/builder

# electron-builder uses Xenial because of native node deps, but for snap Bionic must be used (as we use base: core18)
FROM buildpack-deps:bionic-curl

# this package is used for snapcraft and we should not clear apt list - to avoid apt-get update during snap build

ENV DEBIAN_FRONTEND noninteractive

# Grab dependencies
# Grab the core snap from the stable channel and unpack it in the proper place
RUN apt-get update -qq && apt-get dist-upgrade -qq && apt-get install -qq jq squashfs-tools && \
  curl -L $(curl -H 'X-Ubuntu-Series: 16' 'https://api.snapcraft.io/api/v1/snaps/details/core' | jq '.download_url' -r) --output core.snap && mkdir -p /snap/core && unsquashfs -d /snap/core/current core.snap && \
  curl -L $(curl -H 'X-Ubuntu-Series: 16' 'https://api.snapcraft.io/api/v1/snaps/details/snapcraft?channel=stable' | jq '.download_url' -r) --output snapcraft.snap && mkdir -p /snap/snapcraft && unsquashfs -d /snap/snapcraft/current snapcraft.snap

# Create a snapcraft runner
RUN mkdir -p /snap/bin && echo "#!/bin/sh" > /snap/bin/snapcraft && \
  snap_version="$(awk '/^version:/{print $2}' /snap/snapcraft/current/meta/snap.yaml)" && echo "export SNAP_VERSION=\"$snap_version\"" >> /snap/bin/snapcraft && \
  echo 'exec "$SNAP/usr/bin/python3" "$SNAP/bin/snapcraft" "$@"' >> /snap/bin/snapcraft && \
  chmod +x /snap/bin/snapcraft

# python for node-gyp IS NOT installed, because it is not safe to build some NPM packages on server (as NPM package registry is not trusted)
# rpm is required for FPM to build rpm package
# libsecret-1-dev and libgnome-keyring-dev are required even for prebuild keytar
# do not use  --no-install-recommends - because snapcraft will install with default flags (so, we should also do so to ensure to avoid fetching during build)
# binutils for deb (ar command)
RUN apt-get -qq install bsdtar lzip rpm binutils libopenjp2-tools \
  # snap stage packages
  libnspr4 libnss3 libxss1 libappindicator3-1 libsecret-1-0 && \
  apt-mark hold libnspr4 libnss3 libxss1 libappindicator3-1 libsecret-1-0

RUN curl -L https://nodejs.org/dist/v12.4.0/node-v12.4.0-linux-x64.tar.gz | tar xz -C /usr/local --strip-components=1 && \
  unlink /usr/local/CHANGELOG.md && unlink /usr/local/LICENSE && unlink /usr/local/README.md && \
  # https://github.com/npm/npm/issues/4531
  npm config set unsafe-perm true

COPY node_modules /node_modules

RUN ln -s /node_modules/7zip-bin/linux/x64/7za /usr/local/bin/7za && /node_modules/app-builder-bin/linux/x64/app-builder prefetch-tools && \
  /node_modules/app-builder-bin/linux/x64/app-builder download-electron --configuration '[{"platform":"linux","arch":"x64","version":"5.0.5"}]'

# fix error /usr/local/bundle/gems/fpm-1.5.0/lib/fpm/package/freebsd.rb:72:in `encode': "\xE2" from ASCII-8BIT to UTF-8 (Encoding::UndefinedConversionError)
# http://jaredmarkell.com/docker-and-locales/
# http://askubuntu.com/a/601498
ENV LANG C.UTF-8
ENV LANGUAGE C.UTF-8
ENV LC_ALL C.UTF-8

ENV DEBUG_COLORS true
ENV FORCE_COLOR true

ENV PATH="/snap/bin:$PATH"
ENV SNAP="/snap/snapcraft/current"
ENV SNAP_NAME="snapcraft"
ENV SNAP_ARCH="amd64"

COPY --from=go-builder /builder /builder

CMD ["/builder"]