#!/bin/sh
set -e

export DATA_ROOT_DIR=/tmp
export NGINX_PORT=443
unset REDIS_ENDPOINT
unset DEBUG

export ENV_COLOR=blue
#export ENV_COLOR=green

# https://github.com/moby/moby/issues/30127#issuecomment-302608373
docker-compose -f docker-compose.yml -f docker-compose.prod.yml config --resolve-image-digests > scripts/private/builder.stack.yml
# https://github.com/moby/moby/issues/35532#issuecomment-346753307
sed -i "" 's/8443/443/g' scripts/private/builder.stack.yml

docker-compose -f packages/router/router-compose.yml config --resolve-image-digests > scripts/private/router.stack.yml
sed -i "" 's/8444/443/g' scripts/private/router.stack.yml

node scripts/fix-compose.js