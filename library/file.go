package library

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func CopyDir(dstDir string, srcDir string) error {
	files, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}
		fullName := srcDir + "/" + file.Name()
		dstName := dstDir + "/" + file.Name()
		if file.IsDir() {
			err = CopyDir(dstName, fullName)
		} else {
			_, err = CopyFile(dstName, fullName)
		}
	}
	return nil
}

func CopyFile(dstFileName string, srcFileName string) (written int64, err error) {
	srcFile, err := os.Open(srcFileName)
	if err != nil {
		fmt.Printf("open file error1 = %v\n", err)
	}
	defer srcFile.Close()

	//通过srcFile，获取到READER
	reader := bufio.NewReader(srcFile)

	err = os.MkdirAll(filepath.Dir(dstFileName), os.ModePerm)
	if err != nil {
		//log.Println(err.Error())
		return 0, err
	}
	//打开dstFileName
	dstFile, err := os.OpenFile(dstFileName, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		//fmt.Printf("open file error2 = %v\n", err)
		return
	}

	//通过dstFile，获取到WRITER
	writer := bufio.NewWriter(dstFile)
	//writer.Flush()

	defer dstFile.Close()

	return io.Copy(writer, reader)
}
