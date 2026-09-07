# Phase 12 Mobile App

## Current Build

- Flutter source and Android, iOS, web and Windows wrappers are present under `mobile`.
- Shared Dart API models cover login, signup, workspace, quotes, manufacturing, shipping, payments and messages.
- Customer mobile navigation covers Home, Quotes, Orders, Shipping, Payments and Messages.
- Local development uses backend port `8002`; Android emulators use `10.0.2.2`, while desktop and iOS simulator use `127.0.0.1`.
- The mobile support screen still needs migration from the legacy call-request interaction to the web platform's direct-call contract.

## Toolchain Status

- Git is installed system-wide and is shared safely by separate projects.
- Flutter 3.47.2 is installed under the workspace tools folder and is available on PATH.
- Package resolution, static analysis and widget tests have passed.
- A Flutter web production bundle has built successfully.
- Android SDK setup remains required for APK and emulator builds.
- Windows native builds additionally require Visual Studio with the C++ desktop workload.

## Next Mobile Steps

- Replace legacy call requests with direct ringing, answer, decline and hang-up.
- Add native WebRTC audio using the same signaling contract as the web portal.
- Add image and document picking, compression and upload to the existing protected file API.
- Run on an Android emulator and a physical Android device.
- Add push notifications after backend device-token and notification jobs are ready.
