FROM node:8-slim as builder

WORKDIR /app

COPY package.json yarn.lock ./
RUN yarn --frozen-lockfile
COPY scripts/compile-app.sh /tmp/compile-app.sh
COPY . ./
RUN /tmp/compile-app.sh

# for snapcraft we must use only <= xenial because otherwise not compatible with xenial (glib version error)
# have to use ubuntu instead of alpine as a base because of snapcraft
FROM buildpack-deps:xenial-curl

ENV NODE_VERSION 8.9.1

ENV LANG C.UTF-8
ENV LANGUAGE C.UTF-8
ENV LC_ALL C.UTF-8

# xz-utils only to download node
# do not clean apt lists because snap uses apt to install packages
RUN apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq update && \
  apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq upgrade && \
  apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq --no-install-recommends install unzip snapcraft icnsutils rpm xz-utils git && \
  curl -L https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-x64.tar.xz | tar xJ -C /usr/local --strip-components=1 && \
  unlink /usr/local/CHANGELOG.md && unlink /usr/local/LICENSE && unlink /usr/local/README.md && \
  curl -L https://yarnpkg.com/latest.tar.gz | tar xz -C /usr/local --strip-components=1 && \
  curl -L https://github.com/electron-userland/electron-builder-binaries/releases/download/ran-0.1.3/zstd --output /usr/local/bin/zstd && chmod +x /usr/local/bin/zstd && \
  apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq autoremove && \
  apt-get --option=Dpkg::options::=--force-unsafe-io -option=Dpkg::Use-Pty=0 -qq clean

# snapcraft install a lot of packages to build, so, to avoid it in each build agent, build sample snap to fetch all required depenencies in advance
COPY scripts/snapcraft.yaml /tmp/snap-project/snap/snapcraft.yaml
RUN cd /tmp/snap-project && snapcraft snap && rm -rf /tmp/snap-project

WORKDIR /app

COPY --from=builder /app .

VOLUME /app/certs
EXPOSE 443

CMD [ "node", "/app/out/main.js" ]