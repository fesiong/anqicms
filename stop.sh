#!/bin/bash
### stop
# author fesion
# the bin name is anqicms
APP_NAME=anqicms
APP_PATH="$( cd "$( dirname "$0"  )" && pwd  )"

if pgrep -x "$APP_NAME" >/dev/null
then
    echo "$APP_NAME is running. Stopping it..."
    pkill -9 "$APP_NAME"
    echo "$APP_NAME stopped."
else
    echo "$APP_NAME is not running."
fi
