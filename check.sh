#!/bin/bash
### check 502
# author fesion
# the bin name is goblog
BINNAME=goblog
BINPATH="$( cd "$( dirname "$0"  )" && pwd  )"

# check the pid if exists
exists=`ps -ef | grep '\<goblog\>' |grep -v grep |wc -l`
echo "$(date +'%Y%m%d %H:%M:%S') $BINNAME PID check: $exists" >> $BINPATH/check.log
echo "PID $BINNAME check: $exists"
if [ $exists -eq 0 ]; then
    echo "$BINNAME NOT running"
    cd $BINPATH && nohup $BINPATH/$BINNAME >> $BINPATH/running.log 2>&1 &
fi