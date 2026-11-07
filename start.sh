#!/bin/bash
### check 502
# author fesion
# the bin name is anqicms
APP_NAME=anqicms
APP_PATH="$( cd "$( dirname "$0"  )" && pwd )"

if pgrep -x "$APP_NAME" >/dev/null
then
    echo "$APP_NAME is already running."
else
    echo "$APP_NAME is not running. Starting it..."
    cd $APP_PATH && nohup $APP_PATH/$APP_NAME >> $APP_PATH/running.log 2>&1 &
    echo "$APP_NAME started."
fi
