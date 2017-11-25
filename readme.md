# electron-build-service

Electron Build Service — package Electron applications in a distributable format on any platform for any platform.

Experimental and not fully feature complete. Currently, Linux targets are supported. AppImage and Snap were tested.

Intended only for programmatic usage. Please set option `remoteBuild: true` in the [electron-builder](https://github.com/electron-userland/electron-builder).

## Privacy

Only what your end users see and get, are sent to remote build server. electron-builder, that works with your local project sources on your local machine, packs application in a prepackaged format, that contains only what your end users get on installation/run. The whole project sources are not included and remains only on your local machine.

What is sent to remote build server:
* `info.json` - this file contains project metadata (e.g. version, name) and effective electron-builder configuration.
* prepackaged application — e.g. `linux-unpacked`. This directory in your `dist` and you can inspect what are sent.
* Headers `x-targets` and `x-platform` that contain information what to build. e.g. `{"name": "appImage", "arch": "x64"}`.

Communication is encrypted (TLS). Custom certificate authority is used and required by client (electron-builder), it means that even if someone will take control over domain, build will be rejected due to incorrect certificate (build agent certificate must be issued only by expected certificate authority).

To ensure that you allow using remote build server, option `remoteBuild` is `false` by default in the electron-builder configuration.

## Build Time

Total build time consists of upload, queue waiting, build and download.

Upload and download depends on your internet connection and location. Build servers are located in the USA and EU.
Project packed using [zstd](https://facebook.github.io/zstd/) compression and 50MB will be uploaded in a ~40 seconds for example.

Queue waiting is not predictable for now.

Build time depends on target. 
* AppImage — 20s.
* Snap — 2-3 minutes. AppImage is a default target for electron-builder, so, for now, we are not going to fix Snap build time (it is not electron-builder issue, it is due to snapcraft tool architecture — snap build is slow on a local machine also).
