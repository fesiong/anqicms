package provider

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/response"
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
	return bs.dumpTableSchemaTo(tableName, file)
}

func (bs *BackupStatus) dumpTableSchemaTo(tableName string, writer io.Writer) error {
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
	re, _ = regexp.Compile(` COLLATE=utf8([a-z0-9_]+)(\s|;)?`)
	data = re.ReplaceAllStringFunc(data, func(s string) string {
		if strings.HasSuffix(s, " ") {
			return " "
		}
		return ";"
	})
	if !strings.HasSuffix(data, ";") {
		data += ";"
	}
	_, err = io.WriteString(writer, fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", tableName))
	if err != nil {
		return err
	}
	data = data + "\n\n"
	_, err = io.WriteString(writer, data)
	return err
}

func (bs *BackupStatus) dumpTable(table string, file *os.File) (err error) {
	return bs.dumpTableTo(table, file)
}

func (bs *BackupStatus) dumpTableTo(table string, writer io.Writer) (err error) {
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
				str := fmt.Sprint(d)
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
			_, err = io.WriteString(writer, query)
			if err != nil {
				return err
			}
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
		_, err = io.WriteString(writer, query)
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
	// 备份格式为 .zip，其中包含 database.sql 和 template/ 目录
	backupFile := bs.w.DataPath + "backup/" + time.Now().Format("20060102150405.zip")
	// create dir
	_ = os.MkdirAll(bs.w.DataPath+"backup/", os.ModePerm)
	outFile, err := os.Create(backupFile)
	if err != nil {
		return err
	}
	defer outFile.Close()
	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	t := time.Now()

	tables, err := bs.w.DB.Migrator().GetTables()
	if err != nil {
		return err
	}

	// 先把所有表结构/数据写入一个内存中的 sql 文件
	sqlWriter, err := zipWriter.Create("database.sql")
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
		// dumpTableSchema 直接写入 sqlWriter
		if err = bs.dumpTableSchemaTo(table, sqlWriter); err != nil {
			log.Println(err)
			continue
		}
		// dumpTable 需要逐行写入，这里复用现有逻辑但写入 sqlWriter
		if err = bs.dumpTableTo(table, sqlWriter); err != nil {
			log.Println(err)
			continue
		}
	}

	// 备份当前模板目录
	bs.Message = bs.w.Tr("BackingUpTemplate")
	tplDir := bs.w.GetTemplateDir()
	if tplDir != "" {
		_ = filepath.Walk(tplDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			// 计算相对 zip 内部的路径：template/<相对模板根的路径>
			relPath, err := filepath.Rel(tplDir, path)
			if err != nil {
				return nil
			}
			zipName := "template/" + filepath.ToSlash(relPath)
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return nil
			}
			header.Name = zipName
			header.Method = zip.Deflate
			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			_, _ = io.Copy(writer, f)
			return nil
		})
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
	backupFile, err := bs.w.sanitizeBackupPath(fileName)
	if err != nil {
		bs.Message = err.Error()
		return err
	}

	// 兼容两种格式：旧的 .sql 文件 和新的 .zip 包
	if strings.HasSuffix(fileName, ".zip") {
		return bs.restoreFromZip(backupFile)
	}
	return bs.restoreFromSQL(backupFile)
}

// restoreFromSQL 处理旧的纯 SQL 备份文件
func (bs *BackupStatus) restoreFromSQL(backupFile string) error {
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
		//log.Println("is eof", isEOF)
		tmpStr += line
		if strings.HasSuffix(line, ";\n") || isEOF {
			curSize += int64(len(tmpStr))
			if size > 0 {
				bs.Percent = int(curSize * 100 / size)
			}
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

// restoreFromZip 处理新的 .zip 备份包，包内含 database.sql 和 template/ 目录
func (bs *BackupStatus) restoreFromZip(backupFile string) error {
	zipReader, err := zip.OpenReader(backupFile)
	if err != nil {
		bs.Message = err.Error()
		return err
	}
	defer zipReader.Close()

	// 先还原模板文件
	tplDir := bs.w.GetTemplateDir()
	for _, f := range zipReader.File {
		if !strings.HasPrefix(f.Name, "template/") {
			continue
		}
		// 计算还原到磁盘的目标路径
		relPath := strings.TrimPrefix(f.Name, "template/")
		// 跳过目录条目
		if strings.HasSuffix(f.Name, "/") {
			continue
		}
		targetPath := filepath.Join(tplDir, relPath)
		// 创建父目录
		if mkErr := os.MkdirAll(filepath.Dir(targetPath), os.ModePerm); mkErr != nil {
			log.Println(mkErr)
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			log.Println(openErr)
			continue
		}
		out, createErr := os.Create(targetPath)
		if createErr != nil {
			log.Println(createErr)
			rc.Close()
			continue
		}
		_, _ = io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
	}

	// 再还原数据库
	for _, f := range zipReader.File {
		if f.Name != "database.sql" {
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			return openErr
		}
		defer rc.Close()

		var tmpStr string
		lineReader := bufio.NewReader(rc)
		var size int64 = int64(f.UncompressedSize64)
		var curSize int64 = 0

		isEOF := false
		for {
			line, err := lineReader.ReadString('\n')
			if err != nil {
				log.Println("is restore finished", err)
			}
			if err == io.EOF {
				isEOF = true
			}
			tmpStr += line
			if strings.HasSuffix(line, ";\n") || isEOF {
				curSize += int64(len(tmpStr))
				if size > 0 {
					bs.Percent = int(curSize * 100 / size)
				}
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
	}

	return nil
}

func (w *Website) GetBackupList() []response.BackupInfo {
	files, _ := os.ReadDir(w.DataPath + "backup/")
	var fileList []response.BackupInfo
	for _, file := range files {
		// 兼容旧的 .sql 备份和新的 .zip 备份
		if !strings.HasSuffix(file.Name(), ".sql") && !strings.HasSuffix(file.Name(), ".zip") {
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
			Remark:  w.GetBackupRemark(file.Name()),
		})
	}
	sort.Slice(fileList, func(i, j int) bool {
		first, second := fileList[i], fileList[j]
		return first.LastMod > second.LastMod
	})

	return fileList
}

func (w *Website) sanitizeBackupPath(fileName string) (string, error) {
	if fileName == "" {
		return "", errors.New(w.Tr("BackupFileDoesNotExist"))
	}
	backupDir := filepath.Clean(w.DataPath+"backup/") + string(filepath.Separator)
	cleanPath := filepath.Clean(w.DataPath + "backup/" + fileName)
	if !strings.HasPrefix(cleanPath, backupDir) {
		return "", errors.New(w.Tr("InvalidFilePath"))
	}
	return cleanPath, nil
}

func (w *Website) DeleteBackupData(fileName string) error {
	backupFile, err := w.sanitizeBackupPath(fileName)
	if err != nil {
		return err
	}

	_, err = os.Stat(backupFile)
	if err != nil {
		return err
	}

	err = os.Remove(backupFile)
	if err != nil {
		return err
	}
	// 同步删除备注 sidecar 文件
	_ = os.Remove(backupFile + ".remark.json")

	return nil
}

// GetBackupRemark 读取备份文件对应的备注（sidecar JSON 文件）。
// 文件不存在或解析失败时返回空字符串。
func (w *Website) GetBackupRemark(fileName string) string {
	backupFile, err := w.sanitizeBackupPath(fileName)
	if err != nil {
		return ""
	}
	buf, err := os.ReadFile(backupFile + ".remark.json")
	if err != nil {
		return ""
	}
	var data map[string]string
	if err := json.Unmarshal(buf, &data); err != nil {
		return ""
	}
	return data["remark"]
}

// SetBackupRemark 为备份文件写入备注（sidecar JSON 文件）。
func (w *Website) SetBackupRemark(fileName, remark string) error {
	backupFile, err := w.sanitizeBackupPath(fileName)
	if err != nil {
		return err
	}
	_, err = os.Stat(backupFile)
	if err != nil {
		return err
	}
	data := map[string]string{"remark": remark}
	buf, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(backupFile+".remark.json", buf, os.ModePerm)
}

func (w *Website) ImportBackupFile(file io.Reader, fileName string) error {
	backupFile, err := w.sanitizeBackupPath(fileName)
	if err != nil {
		return err
	}
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
	return w.sanitizeBackupPath(fileName)
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
