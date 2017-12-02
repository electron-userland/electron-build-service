FROM node:8-slim as builder

WORKDIR /project

COPY package.json yarn.lock ./
COPY packages/router/package.json packages/router/
COPY packages/service-registry-redis/package.json packages/service-registry-redis/
RUN yarn --frozen-lockfile

COPY . ./
RUN node_modules/.bin/tsc -p packages/service-registry-redis && node_modules/.bin/tsc -p packages/router && \
  yarn --production --frozen-lockfile && \
  mkdir /app && \
  yarn pack --cwd packages/router --filename /tmp/f.tgz && tar xf /tmp/f.tgz -C /app --strip 1 && \
  unlink node_modules/service-registry-redis && \
  unlink node_modules/router && \
  mv node_modules /app/node_modules && \
  mkdir /app/node_modules/service-registry-redis && yarn pack --cwd packages/service-registry-redis --filename /tmp/f.tgz && tar xf /tmp/f.tgz -C /app/node_modules/service-registry-redis --strip 1

FROM node:8-alpine

RUN apk update && apk add paxctl && paxctl -cm `which node` && apk del paxctl

WORKDIR /app

COPY --from=builder /app .

CMD ["node", "/app/out/router.js"]