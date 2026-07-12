@echo off
rem Remove Explorer context-menu and SendTo integration for the current user.
"%~dp0ayame-diff.exe" shell-uninstall
pause
