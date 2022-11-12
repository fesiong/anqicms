package provider

import "testing"

func TestBackupData(t *testing.T) {
	err := BackupData()

	if err != nil {
		t.Fatal(err)
	}
}

func TestRestoreData(t *testing.T) {
	fileName := "20221111180220.sql"

	err := RestoreData(fileName)
	if err != nil {
		t.Fatal(err)
	}
}
