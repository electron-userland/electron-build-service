FROM node:8-slim as builder

WORKDIR /app

COPY package.json yarn.lock ./
RUN yarn
COPY . ./
RUN yarn compile && yarn --production
RUN rm -rf src && unlink yarn.lock && unlink tsconfig.json && unlink Dockerfile

# have to use debian instead of alpine as a base because of snapcraft
FROM buildpack-deps:xenial-curl

RUN curl -sL https://deb.nodesource.com/setup_8.x | bash - \
  && apt-get install -y --no-install-recommends nodejs unzip snapcraft icnsutils rpm locales \
  && apt-get upgrade -y && apt-get clean && rm -rf /var/lib/apt/lists/* && locale-gen en_US.UTF-8

ENV LANG en_US.UTF-8
ENV LANGUAGE en_US:en
ENV LC_ALL en_US.UTF-8

WORKDIR /app

COPY --from=builder /app .

VOLUME /app/certs
EXPOSE 443

CMD [ "node", "/app/out/main.js" ]