@echo off
setlocal EnableDelayedExpansion
rem 以管理员方式运行cmd
PUSHD %~DP0 & cd /d "%~dp0"
%1 %2
mshta vbscript:createobject("shell.application").shellexecute("%~s0","goto :runas","","runas",1)(window.close)&goto :eof
:runas
rem 尝试启动 AnqiCMS
start /d "D:\phpstudy_pro\WWW\dev.anqicms.com" anqicms.exe
rem 启动服务
exit