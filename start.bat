@echo off
setlocal EnableDelayedExpansion
rem �Թ���Ա��ʽ����cmd
PUSHD %~DP0 & cd /d "%~dp0"
%1 %2
mshta vbscript:createobject("shell.application").shellexecute("%~s0","goto :runas","","runas",1)(window.close)&goto :eof
:runas
rem �������� AnqiCMS
start /d "D:\phpstudy_pro\WWW\dev.anqicms.com" anqicms.exe
rem ��������
exit