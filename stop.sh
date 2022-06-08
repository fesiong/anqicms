#!/bin/bash
### stop
# author fesion
# the bin name is anqicms
BINNAME=anqicms
BINPATH="$( cd "$( dirname "$0"  )" && pwd  )"

# check the pid if exists
exists=`ps -ef | grep '\<anqicms\>' |grep -v grep |awk '{printf $2}'`
echo "$(date +'%Y%m%d %H:%M:%S') $BINNAME PID check: $exists" >> $BINPATH/check.log
echo "PID $BINNAME check: $exists"
if [ $exists -eq 0 ]; then
    echo "$BINNAME NOT running"
else
    echo "$BINNAME is running"
    kill -9 $exists
    echo "$BINNAME is stop"
fi