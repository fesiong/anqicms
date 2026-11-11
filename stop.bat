@echo off
chcp 65001
setlocal EnableDelayedExpansion
rem �Թ�����ʽ����cmd
PUSHD %~DP0 & cd /d "%~dp0"
%1 %2
mshta vbscript:createobject("shell.application").shellexecute("%~s0","goto :runas","","runas",1)(window.close)&goto :eof
:runas
rem �ر� AnqiCMS

taskkill /f /im anqicms.exe
rem �ر� AnqiCMS
pause