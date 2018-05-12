#!/bin/sh
set -e

export DATA_ROOT_DIR=/tmp
export NGINX_PORT=443
unset REDIS_ENDPOINT
unset DEBUG

# https://github.com/moby/moby/issues/30127#issuecomment-302608373
docker-compose -f docker-compose.yml -f docker-compose.prod.yml config --resolve-image-digests > stacks/builder.stack.yml
# https://github.com/moby/moby/issues/35532#issuecomment-346753307
sed -i "" 's/8443/443/g' stacks/builder.stack.yml
sed -i "" 's/8444/443/g' stacks/builder.stack.yml

#node scripts/fix-compose.js