package repository

import (
	"encoding/json"
	"errors"
	"os"

	"z-mihomo-health-check/internal/domain"
)

func LoadSitePool(filename string) (*domain.SitePoolFile, error) {
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("无法读取全局站点池状态")
	}
	var saved domain.SitePoolFile
	if json.Unmarshal(data, &saved) != nil || saved.Version != 1 || saved.Sites == nil {
		return nil, errors.New("全局站点池状态文件无效")
	}
	return &saved, nil
}
func SaveSitePool(filename string, states map[string]domain.SiteHealth) error {
	data, err := json.Marshal(domain.SitePoolFile{Version: 1, Sites: states})
	if err != nil {
		return err
	}
	return WriteHistoryFile(filename, data)
}
