# All build caches/temp on D: — source before Go, npm, Flutter, Gradle.
#   . "D:\Factory Software\ChakuChuri Platform\scripts\local-build-env.ps1"

$cacheRoot = 'D:\DevCache'
$toolsRoot = 'D:\Factory Software\tools'

$env:TEMP = Join-Path $cacheRoot 'tmp'
$env:TMP = $env:TEMP
$env:TMPDIR = $env:TEMP
$env:GOTMPDIR = Join-Path $cacheRoot 'go-tmp'
$env:GOCACHE = Join-Path $cacheRoot 'go-build'
$env:GOMODCACHE = Join-Path $cacheRoot 'go-mod'
$env:npm_config_cache = Join-Path $cacheRoot 'npm-cache'
$env:PUB_CACHE = Join-Path $cacheRoot 'pub-cache'
$env:GRADLE_USER_HOME = Join-Path $cacheRoot 'gradle'
$env:ANDROID_HOME = Join-Path $toolsRoot 'android-sdk'
$env:ANDROID_SDK_ROOT = $env:ANDROID_HOME
$env:FLUTTER_ROOT = Join-Path $toolsRoot 'flutter'
# Keep Dart analysis / tool state off C:
$env:ANALYZER_STATE_LOCATION_OVERRIDE = Join-Path $cacheRoot 'dart-analyzer'
# Do NOT set JAVA_TOOL_OPTIONS — PowerShell treats its stderr banner as a terminating error.
# Java tmpdir is set via mobile/android/gradle.properties (-Djava.io.tmpdir=D:/DevCache/tmp).
Remove-Item Env:JAVA_TOOL_OPTIONS -ErrorAction SilentlyContinue

New-Item -ItemType Directory -Force -Path @(
  $env:TEMP,
  $env:GOTMPDIR,
  $env:GOCACHE,
  $env:GOMODCACHE,
  $env:npm_config_cache,
  $env:PUB_CACHE,
  $env:GRADLE_USER_HOME,
  $env:ANALYZER_STATE_LOCATION_OVERRIDE
) | Out-Null

$flutterBin = Join-Path $env:FLUTTER_ROOT 'bin'
if ($env:Path -notlike "*$flutterBin*") {
  $env:Path = "$flutterBin;$env:Path"
}

Write-Host "Build env -> D:\DevCache (TEMP/PUB/GRADLE/GO/npm). C: not used for caches."
