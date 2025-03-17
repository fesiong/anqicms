package provider

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/response"
	"log"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const ChunkSizeInMB = 16
const MaxStmtSize = 1000000

const (
	BackupTypeBackup  = "backup"
	BackupTypeRestore = "restore"
)

type BackupStatus struct {
	w        *Website
	Finished bool   `json:"finished"` // true | false
	Type     string `json:"type"`     // type = backup|restore
	Percent  int    `json:"percent"`  // 0-100
	Message  string `json:"message"`  // current message
}

var backupStatus *BackupStatus

func (w *Website) GetBackupStatus() *BackupStatus {
	return backupStatus
}

func (w *Website) NewBackup() (*BackupStatus, error) {
	if backupStatus != nil && backupStatus.Finished == false {
		return nil, errors.New(w.Tr("TaskIsRunningPleaseWait"))
	}

	backupStatus = &BackupStatus{
		w:        w,
		Finished: false,
		Percent:  0,
		Message:  "",
	}

	return backupStatus, nil
}

func (bs *BackupStatus) dumpTableSchema(tableName string, file *os.File) error {
	var data string
	err := bs.w.DB.Raw(fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", bs.w.Mysql.Database, tableName)).Row().Scan(&tableName, &data)
	if err != nil {
		return err
	}

	// 移除 CHARACTER SET utf8mb4 COLLATE utf8mb4_latvian_ci NOT NULL DEFAULT '',
	// 移除 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

	re, _ := regexp.Compile(` COLLATE utf8([a-z0-9_]+)(\s|,)`)
	data = re.ReplaceAllStringFunc(data, func(s string) string {
		if strings.HasSuffix(s, " ") {
			return " "
		}
		return ","
	})
	re, _ = regexp.Compile(` COLLATE=utf8([a-z0-9_]+)(\s|;)`)
	data = re.ReplaceAllStringFunc(data, func(s string) string {
		if strings.HasSuffix(s, " ") {
			return " "
		}
		return ";"
	})

	_, err = file.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", tableName))
	data = data + "\n\n"
	_, err = file.WriteString(data)
	return err
}

func (bs *BackupStatus) dumpTable(table string, file *os.File) (err error) {
	var allBytes uint64
	var allRows uint64

	cursor, err := bs.w.DB.Raw(fmt.Sprintf("SELECT * FROM `%s`.`%s`", bs.w.Mysql.Database, table)).Rows()
	if err != nil {
		return err
	}
	defer func() {
		err = cursor.Close()
	}()
	colTypes, err := cursor.ColumnTypes()
	if err != nil {
		return err
	}
	cols, err := cursor.Columns()
	if err != nil {
		return err
	}
	destColNames := bs.w.DB.Statement.Quote(cols)
	stmtSize := 0
	chunkBytes := 0
	rows := make([]string, 0, 256)
	inserts := make([]string, 0, 256)
	for cursor.Next() {
		var dest map[string]interface{}
		err = bs.w.DB.ScanRows(cursor, &dest)
		if err != nil {
			return err
		}
		values := make([]string, 0, 16)
		for i, c := range cols {
			d, ok := dest[c]
			if !ok || d == nil {
				values = append(values, "NULL")
			} else {
				str := fmt.Sprintf("%v", d)
				switch reflect.TypeOf(d).Kind() {
				case reflect.Int16, reflect.Int8, reflect.Int32, reflect.Int64, reflect.Int, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
					values = append(values, str)
				case reflect.String:
					values = append(values, fmt.Sprintf("'%s'", library.EscapeString(str)))
				default:
					colType := colTypes[i]
					if strings.Contains(colType.DatabaseTypeName(), "DATE") || strings.Contains(colType.DatabaseTypeName(), "TIME") {
						str = str[0:strings.Index(str, " +")]
					}
					values = append(values, fmt.Sprintf("'%s'", str))
				}
			}
		}

		r := "(" + strings.Join(values, ",") + ")"
		rows = append(rows, r)

		allRows++
		stmtSize += len(r)
		chunkBytes += len(r)
		allBytes += uint64(len(r))

		if stmtSize >= MaxStmtSize {
			insertOne := fmt.Sprintf("INSERT INTO `%s`%s VALUES\n%s", table, destColNames, strings.Join(rows, ",\n"))
			inserts = append(inserts, insertOne)
			rows = rows[:0]
			stmtSize = 0
		}

		if (chunkBytes / 1024 / 1024) >= ChunkSizeInMB {
			query := strings.Join(inserts, ";\n") + ";\n"
			_, err = file.WriteString(query)
			inserts = inserts[:0]
			chunkBytes = 0
		}
	}
	if chunkBytes > 0 {
		if len(rows) > 0 {
			insertOne := fmt.Sprintf("INSERT INTO `%s`%s VALUES\n%s", table, destColNames, strings.Join(rows, ",\n"))
			inserts = append(inserts, insertOne)
		}

		query := strings.Join(inserts, ";\n") + ";\n"
		_, err = file.WriteString(query)
	}

	return nil
}

func (bs *BackupStatus) BackupData() error {
	bs.Type = BackupTypeBackup
	bs.Percent = 0
	defer func() {
		bs.Finished = true
		time.AfterFunc(3*time.Second, func() {
			if bs.Finished {
				backupStatus = nil
			}
		})
	}()
	backupFile := bs.w.DataPath + "backup/" + time.Now().Format("20060102150405.sql")
	// create dir
	_ = os.MkdirAll(bs.w.DataPath+"backup/", os.ModePerm)
	outFile, err := os.Create(backupFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	t := time.Now()

	tables, err := bs.w.DB.Migrator().GetTables()
	if err != nil {
		return err
	}

	for _, table := range tables {
		if bs.Percent < 99 {
			bs.Percent++
		}
		bs.Message = bs.w.Tr("BackingUp%s", table)
		// 跳过logs表
		if strings.Contains(table, "_logs") {
			continue
		}
		err = bs.dumpTableSchema(table, outFile)
		if err != nil {
			log.Println(err)
			continue
		}

		err = bs.dumpTable(table, outFile)
		if err != nil {
			log.Println(err)
			continue
		}
	}

	log.Printf("dumping.all.done.cost[%s], elapsed", time.Since(t).String())

	return nil
}

func (bs *BackupStatus) RestoreData(fileName string) error {
	bs.Type = BackupTypeRestore
	bs.Percent = 0
	defer func() {
		bs.Finished = true
		time.AfterFunc(3*time.Second, func() {
			if bs.Finished {
				backupStatus = nil
			}
		})
	}()
	if fileName == "" {
		bs.Message = bs.w.Tr("BackupFileDoesNotExist")
		return errors.New(bs.w.Tr("BackupFileDoesNotExist"))
	}
	backupFile := bs.w.DataPath + "backup/" + fileName
	outFile, err := os.Open(backupFile)
	if err != nil {
		bs.Message = err.Error()
		return err
	}
	defer outFile.Close()

	var tmpStr string
	lineReader := bufio.NewReader(outFile)
	var size int64 = 0
	var curSize int64 = 0
	stat, err := outFile.Stat()
	if err == nil {
		size = stat.Size()
	}

	isEOF := false
	for {
		line, err := lineReader.ReadString('\n')
		if err != nil {
			log.Println("is restore finished", err)
		}
		if err == io.EOF {
			isEOF = true
		}
		log.Println("is eof", isEOF)
		tmpStr += line
		if strings.HasSuffix(line, ";\n") || isEOF {
			curSize += int64(len(tmpStr))
			bs.Percent = int(curSize * 100 / size)
			if strings.HasPrefix(tmpStr, "DROP TABLE") {
				re, _ := regexp.Compile("`(.+?)`")
				match := re.FindStringSubmatch(tmpStr)
				if len(match) == 2 {
					bs.Message = bs.w.Tr("RestoringData%s", match[1])
				}
			}
			// 跳过logs表
			var checkStr string
			lnIndex := strings.Index(tmpStr, "\n")
			if lnIndex > 0 {
				checkStr = tmpStr[0:lnIndex]
			} else {
				checkStr = tmpStr
			}
			if !strings.Contains(checkStr, "_logs`") {
				bs.w.DB.Exec(tmpStr)
			}
			tmpStr = ""
		}
		if isEOF {
			break
		}
	}

	return nil
}

func (w *Website) GetBackupList() []response.BackupInfo {
	files, _ := os.ReadDir(w.DataPath + "backup/")
	var fileList []response.BackupInfo
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		info, err := file.Info()
		if err != nil {
			continue
		}
		fileList = append(fileList, response.BackupInfo{
			Name:    file.Name(),
			LastMod: info.ModTime().Unix(),
			Size:    info.Size(),
		})
	}
	sort.Slice(fileList, func(i, j int) bool {
		first, second := fileList[i], fileList[j]
		return first.LastMod > second.LastMod
	})

	return fileList
}

func (w *Website) DeleteBackupData(fileName string) error {
	if fileName == "" {
		return errors.New(w.Tr("BackupFileDoesNotExist"))
	}
	fileName = strings.ReplaceAll(fileName, "..", "")
	fileName = strings.ReplaceAll(fileName, "\\", "")
	backupFile := w.DataPath + "backup/" + fileName

	_, err := os.Stat(backupFile)
	if err != nil {
		return err
	}

	err = os.Remove(backupFile)

	return err
}

func (w *Website) ImportBackupFile(file io.Reader, fileName string) error {
	fileName = strings.ReplaceAll(fileName, "..", "")
	fileName = strings.ReplaceAll(fileName, "\\", "")
	backupFile := w.DataPath + "backup/" + fileName
	// create dir
	_ = os.MkdirAll(w.DataPath+"backup/", os.ModePerm)

	outFile, err := os.Create(backupFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	l, err := io.Copy(outFile, file)
	log.Println("copydata", l, err)

	return err
}

func (w *Website) GetBackupFilePath(fileName string) (string, error) {
	if fileName == "" {
		return "", errors.New(w.Tr("BackupFileDoesNotExist"))
	}
	fileName = strings.ReplaceAll(fileName, "..", "")
	fileName = strings.ReplaceAll(fileName, "\\", "")
	backupFile := w.DataPath + "backup/" + fileName

	_, err := os.Stat(backupFile)
	if err != nil {
		return "", err
	}

	return backupFile, nil
}

func (w *Website) CleanupWebsiteData(cleanUploads bool) {
	t := time.Now()

	tables, err := w.DB.Migrator().GetTables()
	if err != nil {
		return
	}

	for _, table := range tables {
		// 排除几个表
		if table == "admin_groups" ||
			table == "admin_login_logs" ||
			table == "admin_logs" ||
			table == "admins" ||
			table == "settings" ||
			table == "websites" {
			continue
		}
		err = w.DB.Exec(fmt.Sprintf("TRUNCATE `%s`.`%s`", w.Mysql.Database, table)).Error
		if err != nil {
			log.Println(err)
			continue
		}
	}
	if cleanUploads {
		_ = os.RemoveAll(w.PublicPath + "uploads/")
	}
	// 清理cache
	w.DeleteCache()
	w.RemoveHtmlCache()
	// 重新初始化
	w.InitModelData()

	log.Printf("清空整站数据.用时[%s]", time.Since(t).String())
}
