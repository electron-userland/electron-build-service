FROM node:8-slim as builder

WORKDIR /project

COPY package.json yarn.lock ./
COPY packages/electron-build-server/package.json packages/electron-build-server/
COPY packages/service-registry-redis/package.json packages/service-registry-redis/
RUN yarn --frozen-lockfile

COPY . ./
RUN yarn compile && \
  yarn --production --frozen-lockfile && \
  mkdir /app && \
  yarn pack --cwd packages/electron-build-server --filename /tmp/f.tgz && tar xf /tmp/f.tgz -C /app --strip 1 && \
  unlink node_modules/service-registry-redis && \
  unlink node_modules/electron-build-server && \
  mv node_modules /app/node_modules && \
  mkdir /app/node_modules/service-registry-redis && yarn pack --cwd packages/service-registry-redis --filename /tmp/f.tgz && tar xf /tmp/f.tgz -C /app/node_modules/service-registry-redis --strip 1

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