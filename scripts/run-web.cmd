@echo off
title ChakuChuri WEB :5170 - KEEP OPEN
cd /d "D:\Factory Software\ChakuChuri Platform\frontend"
echo ChakuChuri Web on http://192.168.10.2:5170
echo Do NOT close this window while using the software.
call npm run dev -- --host 0.0.0.0 --port 5170
echo.
echo Web stopped. Press any key to close.
pause >nul
