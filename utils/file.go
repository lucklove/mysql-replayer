package utils

import (
	"io"
	"os"
	"fmt"
	"errors"
)

func InsertSQLToFile(file *os.File, sql string, timestamp int64) {
	file.WriteString(fmt.Sprintf("%d\n", timestamp))
	file.WriteString(fmt.Sprintf("%d\n", len(sql)))
	file.WriteString(fmt.Sprintf("%s\n", sql))
}

func EnsureDir(path string) error {
	if fi, err := os.Stat(path); os.IsNotExist(err) {
		return os.Mkdir(path, 0755)
	} else if err != nil {
		return err
	} else if !fi.IsDir() {
		return errors.New(fmt.Sprintf("%s is not a directory", path))
	} else {
		return nil
	}
}

func LogIOError(err error) {
	if err != io.EOF {
		fmt.Println(err)
	}
}