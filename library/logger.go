package library

import (
	"fmt"
	"os"
	"sync"
)

var gRWLock *sync.RWMutex

func DebugLog(cachePath, name string, v ...interface{}) {
	filePath := cachePath + name
	logFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
	if nil != err {
		//打开失败，不做记录
		return
	}
	defer logFile.Close()
	gRWLock.Lock()
	logFile.WriteString(fmt.Sprintln(v...))
	gRWLock.Unlock()
}

func init() {
	gRWLock = new(sync.RWMutex)
}
