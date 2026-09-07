# ChakuChuri Mobile

Flutter customer app for the ChakuChuri.pk platform.

## Current Status

- Login and signup connect to the same backend auth envelope as the web portal.
- Customer workspace screens use `GET /api/workspace`.
- Quote, shipping, payment proof record, message and call request actions use the active Go backend APIs.
- Native in-app audio is intentionally isolated for the next mobile pass; the current app already shows call waiting/live-call state from the backend.
- Static analysis, widget tests and a production web build pass.

## Local Setup

Flutter 3.47.2 is installed at:

```powershell
D:\Factory Software\tools\flutter
```

The platform wrappers are already generated. Use these commands from this folder:

```powershell
cd "D:\Factory Software\ChakuChuri Platform\mobile"
"D:\Factory Software\tools\flutter\bin\flutter.bat" pub get
"D:\Factory Software\tools\flutter\bin\flutter.bat" analyze
"D:\Factory Software\tools\flutter\bin\flutter.bat" test
"D:\Factory Software\tools\flutter\bin\flutter.bat" run -d chrome
```

Android builds require Android Studio/SDK and accepted Android licenses. Windows desktop builds require Visual Studio with the Desktop development with C++ workload.

Local backend defaults:

- Android emulator: `http://10.0.2.2:8002`
- iOS simulator / desktop: `http://127.0.0.1:8002`
- Physical phone: use your computer LAN IP, for example `http://192.168.1.10:8002`
