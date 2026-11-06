package repository

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"z-mihomo-health-check/internal/config"
)

func LoadConfig(filename string) (config.Config, error) {
	c := config.DefaultConfig()
	b, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, errors.New("无法读取配置文件")
	}
	if json.Unmarshal(b, &c) != nil {
		return c, errors.New("配置文件不是有效 JSON")
	}
	return c, c.Validate()
}
func SaveConfig(filename string, c config.Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
		return errors.New("无法创建数据目录")
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(filename), ".config-*")
	if err != nil {
		return errors.New("无法创建配置临时文件")
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(b, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return errors.New("写入配置失败")
	}
	if os.Rename(name, filename) != nil {
		return errors.New("替换配置文件失败")
	}
	return nil
}
