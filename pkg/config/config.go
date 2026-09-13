package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig 应用配置
type AppConfig struct {
	ServerRoot     string            `json:"server_root"`      // 服务端根目录
	EngineType     int               `json:"engine_type"`      // 引擎类型
	MonItemsDir    string            `json:"mon_items_dir"`    // MonItems目录
	LastOpenDir    string            `json:"last_open_dir"`    // 上次打开的目录
	SimConfig      SimConfigData     `json:"sim_config"`       // 模拟配置
	Theme          string            `json:"theme"`            // 主题 light/dark
	Language       string            `json:"language"`         // 语言
	AutoBackup     bool              `json:"auto_backup"`      // 自动备份
}

// SimConfigData 模拟配置持久化
type SimConfigData struct {
	DurationHours   float64 `json:"duration_hours"`
	KillRatio       float64 `json:"kill_ratio"`
	RefreshInterval float64 `json:"refresh_interval"`
	RefreshCount    int     `json:"refresh_count"`
}

// DefaultConfig 默认配置
func DefaultConfig() *AppConfig {
	return &AppConfig{
		Theme:      "dark",
		Language:   "zh-CN",
		AutoBackup: true,
		SimConfig: SimConfigData{
			DurationHours:   1,
			KillRatio:       0.6,
			RefreshInterval: 60,
			RefreshCount:    10,
		},
	}
}

// configPath 获取配置文件路径
func configPath() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "config.json")
}

// Load 加载配置
func Load() (*AppConfig, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), nil
	}
	return cfg, nil
}

// Save 保存配置
func Save(cfg *AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0644)
}
