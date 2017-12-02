const { createRedisClient, ServiceCatalog } = require("service-registry-redis")

async function main() {
  const redisClient = await createRedisClient()
  const catalog = new ServiceCatalog(redisClient)
  console.log(JSON.stringify(await catalog.getServices(), null, 2))
}

main()
  .catch(error => {
    console.error(error.stack || error)
    process.exit(1)
  })