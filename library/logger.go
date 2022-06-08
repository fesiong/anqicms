package library

import (
	"fmt"
	"kandaoni.com/anqicms/config"
	"os"
	"sync"
)

var gRWLock *sync.RWMutex

func DebugLog(name string, v ...interface{}) {
	filePath := fmt.Sprintf("%scache/%s.log", config.ExecPath, name)
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