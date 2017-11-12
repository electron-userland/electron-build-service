const {execFileSync} = require("child_process")

const {snapshots} = JSON.parse(execFileSync("curl", ["--silent", "--show-error", "--header", "Content-Type: application/json", "--header", `Authorization: Bearer ${process.argv[2]}`, "https://api.digitalocean.com/v2/snapshots"]), {
  maxBuffer: 1024 * 1024,
})

const snapshot = findServiceSnapshot(snapshots)
if (snapshot == null) {
  process.stderr.write("Cannot find electron-build-service snapshot")
  process.exitCode = 1
}
else {
  process.stdout.write(snapshot.id)
}

function findServiceSnapshot(snapshots) {
  // sorted from oldest to newest
  for (let i = snapshots.length - 1; i >= 0; i--) {
    const snapshot = snapshots[i]
    if (snapshot.name.startsWith("electron-build-service-")) {
      return snapshot
    }
  }

  return null
}