package main

import (
	"os"

	"github.com/dgraph-io/badger"
)

func compareLocalFile(updateDB *badger.DB, path string, file File) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	if stat.Size() != file.Size {
		return false
	}
	dbTxn := updateDB.NewTransaction(false)
	defer dbTxn.Commit()
	item, err := dbTxn.Get([]byte("ModifiedTime_" + path))
	if err != nil {
		return false
	}
	return item.String() == file.ModifiedAt
}

func updateLocalFileDB(updateDB *badger.DB, path string, file File) {
	dbTxn := updateDB.NewTransaction(true)
	defer dbTxn.Commit()
	dbTxn.Set([]byte("ModifiedTime_"+path), []byte(file.ModifiedAt))
}
