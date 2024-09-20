package provider

import "testing"

func (w *Website) TestBackupData(t *testing.T) {
	err := w.BackupData()

	if err != nil {
		t.Fatal(err)
	}
}

func (w *Website) TestRestoreData(t *testing.T) {
	fileName := "20221111180220.sql"

	err := w.RestoreData(fileName)
	if err != nil {
		t.Fatal(err)
	}
}
