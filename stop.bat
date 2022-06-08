@echo off
chcp 65001
setlocal EnableDelayedExpansion
rem 以管理方式运行cmd
PUSHD %~DP0 & cd /d "%~dp0"
%1 %2
mshta vbscript:createobject("shell.application").shellexecute("%~s0","goto :runas","","runas",1)(window.close)&goto :eof
:runas
rem 关闭 AnqiCMS

taskkill /f /im anqicms.exe
rem 关闭 AnqiCMS
pause