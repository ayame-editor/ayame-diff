@echo off
setlocal
cd /d "%~dp0"
ayame-diff.exe --interactive
if errorlevel 1 pause
