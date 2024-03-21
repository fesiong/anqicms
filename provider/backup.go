package provider

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"kandaoni.com/anqicms/library"
	"kandaoni.com/anqicms/response"
	"log"
	"mime/multipart"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"
)

const ChunkSizeInMB = 16
const MaxStmtSize = 1000000

func (w *Website) dumpTableSchema(tableName string, file *os.File) error {
	var data string
	err := w.DB.Raw(fmt.Sprintf("SHOW CREATE TABLE `%s`.`%s`", w.Mysql.Database, tableName)).Row().Scan(&tableName, &data)
	if err != nil {
		return err
	}

	_, err = file.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", tableName))
	data = data + ";\n\n"
	_, err = file.WriteString(data)
	return err
}

func (w *Website) dumpTable(table string, file *os.File) (err error) {
	var allBytes uint64
	var allRows uint64

	cursor, err := w.DB.Raw(fmt.Sprintf("SELECT * FROM `%s`.`%s`", w.Mysql.Database, table)).Rows()
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
	destColNames := w.DB.Statement.Quote(cols)
	stmtSize := 0
	chunkBytes := 0
	rows := make([]string, 0, 256)
	inserts := make([]string, 0, 256)
	for cursor.Next() {
		var dest map[string]interface{}
		err = w.DB.ScanRows(cursor, &dest)
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

func (w *Website) BackupData() error {
	backupFile := w.DataPath + "backup/" + time.Now().Format("20060102150405.sql")
	// create dir
	_ = os.MkdirAll(w.DataPath+"backup/", os.ModePerm)
	outFile, err := os.Create(backupFile)
	if err != nil {
		return err
	}

	t := time.Now()

	tables, err := w.DB.Migrator().GetTables()
	if err != nil {
		return err
	}

	for _, table := range tables {
		err = w.dumpTableSchema(table, outFile)
		if err != nil {
			log.Println(err)
			continue
		}

		err = w.dumpTable(table, outFile)
		if err != nil {
			log.Println(err)
			continue
		}
	}

	log.Printf("dumping.all.done.cost[%s], elapsed", time.Since(t).String())

	return nil
}

func (w *Website) RestoreData(fileName string) error {
	if fileName == "" {
		return errors.New("备份文件不存在")
	}
	backupFile := w.DataPath + "backup/" + fileName
	outFile, err := os.Open(backupFile)
	if err != nil {
		return err
	}
	defer outFile.Close()

	var tmpStr string
	lineReader := bufio.NewReader(outFile)
	for {
		line, err := lineReader.ReadString('\n')
		if err == io.EOF {
			break
		}
		tmpStr += line
		if strings.HasSuffix(line, ";\n") {
			w.DB.Exec(tmpStr)
			tmpStr = ""
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
		return errors.New("备份文件不存在")
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

func (w *Website) ImportBackupFile(file multipart.File, fileName string) error {
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

	_, err = io.Copy(outFile, file)

	return err
}

func (w *Website) GetBackupFilePath(fileName string) (string, error) {
	if fileName == "" {
		return "", errors.New("备份文件不存在")
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
