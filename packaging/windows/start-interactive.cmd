@echo off
setlocal
cd /d "%~dp0"
fcsv-diff.exe --interactive
if errorlevel 1 pause
