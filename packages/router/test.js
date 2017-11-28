const sortList = require("./out/router").sortList
const assert = require("assert")

const a = {cpuCount: 3, jobCount: 3}
const b = {cpuCount: 3, jobCount: 3}
assert.deepEqual(sortList([a, b]), [a, b])

assert.deepEqual(sortList([a]), [a])
assert.deepEqual(sortList([a, {...a}]), [{...a}, {...a}])

assert.deepEqual(sortList([{cpuCount: 3, jobCount: 3}, {cpuCount: 3, jobCount: 0}]), [{cpuCount: 3, jobCount: 0}])
assert.deepEqual(sortList([{cpuCount: 1, jobCount: 1}, {cpuCount: 2, jobCount: 1}]), [{cpuCount: 2, jobCount: 1}])