package repository

import (
	"errors"
	"os"
	"path/filepath"
)

func WriteHistoryFile(filename string, data []byte) error {
	if os.MkdirAll(filepath.Dir(filename), 0700) != nil {
		return errors.New("无法创建历史记录目录")
	}
	f, err := os.CreateTemp(filepath.Dir(filename), ".history-*")
	if err != nil {
		return errors.New("无法创建历史记录临时文件")
	}
	temporary := f.Name()
	defer os.Remove(temporary)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return errors.New("历史记录写入失败")
	}
	if os.Rename(temporary, filename) != nil {
		return errors.New("历史记录文件替换失败")
	}
	return nil
}
