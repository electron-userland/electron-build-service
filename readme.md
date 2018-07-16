# electron-build-service

Electron Build Service — package Electron applications in a distributable format on any platform for any platform.

Experimental and not fully feature complete. Currently, Linux targets are supported.

Intended only for programmatic usage.

Free public Electron Build Service is provided and used by electron-builder since 19.48.0.

This readme contains information not about electron-build-server, but about free public electron build service. Self-hosted usage documentation can be added by request. 

## Why

* Free public service for easier packaging Electron applications on any OS for any OS.
* No need to setup build environment and copy project sources. Just run server and that's all (if for some reasons you cannot use free service).
* Much faster builds compared to CI servers because no need to checkout project sources, build it and so on.

Please note — it is not Electron issue that you cannot build app for all platforms [on one platform](https://www.electron.build/multi-platform-build).

## Pricing

Service not going to be monetized. But build servers costs money. [Donations](https://donorbox.org/electron-build-service) welcome. Do not forget to specify your app name in the form — your donation will affect build time not only indirectly (new build server for all), but directly — specified apps will have higher priority.

## Privacy

Only what your end users see and get, are sent to remote build server. electron-builder, that works with your local project sources on your local machine, packs application in a prepackaged format, that contains only what your end users get on installation/run. The whole project sources are not included and remains only on your local machine.

What is sent to remote build server:
* `info.json` - this file contains project metadata (e.g. version, name) and effective electron-builder configuration.
* prepackaged application — e.g. `linux-unpacked`. This directory in your `dist` and you can inspect what are sent.
* Headers `x-targets` and `x-platform` that contain information what to build. e.g. `{"name": "appImage", "arch": "x64"}`.

Communication is encrypted. Custom certificate authority is used and required by client (electron-builder), it means that even if someone will take control over domain, build will be rejected due to incorrect certificate (build agent certificate must be issued only by expected certificate authority).

To disable using Electron Build Service set option `remoteBuild: false` in the [electron-builder](https://github.com/electron-userland/electron-builder).

### Server Locations

* Europe — Amsterdam and Frankfurt.
* Canada — Beauharnois.

Which build server will be used is not predictable. Your actual location is not used for now to select build server, but planned (by IP address).

See [Cloud Hosting Choice](cloud-hosting-choice.md) about used providers.

## Build Time

Total build time consists of upload, queue waiting, build and download.

Upload depends on your CPU, internet connection and location.
Download depends on your internet connection (location doesn't matter).
Project packed using [zstd](https://facebook.github.io/zstd/) compression and 50MB will be uploaded in a ~20 seconds for example.

Queue waiting is not predictable for now. In the future build agents will be started on demand.

Build time depends on target. 
* AppImage — 10s. 
* deb — 70s.
* Snap — 1 minute 50 second.