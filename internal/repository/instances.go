package repository

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"z-mihomo-health-check/internal/domain"
)

type InstanceRegistry struct {
	Version   int                   `json:"version"`
	Instances []domain.InstanceInfo `json:"instances"`
}

func LoadRegistry(root string) (*InstanceRegistry, error) {
	data, err := os.ReadFile(filepath.Join(root, "instances.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("无法读取实例列表")
	}
	var registry InstanceRegistry
	if json.Unmarshal(data, &registry) != nil {
		return nil, errors.New("实例列表不是有效 JSON")
	}
	if registry.Version != 1 || len(registry.Instances) == 0 {
		return nil, errors.New("实例列表版本无效或没有实例")
	}
	return &registry, nil
}
func SaveRegistry(root string, items []domain.InstanceInfo) error {
	data, err := json.MarshalIndent(InstanceRegistry{Version: 1, Instances: items}, "", "  ")
	if err != nil || WriteHistoryFile(filepath.Join(root, "instances.json"), data) != nil {
		return domain.InstanceFailure(500, "无法保存实例列表")
	}
	return nil
}
