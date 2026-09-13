package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Manager 备份管理器
type Manager struct {
	BackupDir string
}

// New 创建备份管理器
func New(serverRoot string) *Manager {
	backupDir := filepath.Join(serverRoot, "Envir", "MonItems_Backup")
	return &Manager{BackupDir: backupDir}
}

// BackupFile 备份单个文件
func (m *Manager) BackupFile(filePath string) (string, error) {
	if err := os.MkdirAll(m.BackupDir, 0755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取源文件失败: %w", err)
	}

	baseName := filepath.Base(filePath)
	ext := filepath.Ext(baseName)
	nameWithoutExt := baseName[:len(baseName)-len(ext)]
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)
	backupPath := filepath.Join(m.BackupDir, backupName)

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("写入备份文件失败: %w", err)
	}

	return backupPath, nil
}

// BackupDirectory 备份整个MonItems目录
func (m *Manager) BackupDirectory(dirPath string) (string, error) {
	timestamp := time.Now().Format("20060102_150405")
	backupSubDir := filepath.Join(m.BackupDir, "full_"+timestamp)
	if err := os.MkdirAll(backupSubDir, 0755); err != nil {
		return "", err
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".txt" {
			continue
		}
		src := filepath.Join(dirPath, entry.Name())
		dst := filepath.Join(backupSubDir, entry.Name())
		data, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		os.WriteFile(dst, data, 0644)
	}

	return backupSubDir, nil
}

// ListBackups 列出备份文件
func (m *Manager) ListBackups() ([]string, error) {
	if _, err := os.Stat(m.BackupDir); os.IsNotExist(err) {
		return nil, nil
	}

	var backups []string
	entries, err := os.ReadDir(m.BackupDir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		backups = append(backups, entry.Name())
	}
	return backups, nil
}

// RestoreFile 从备份恢复文件
func (m *Manager) RestoreFile(backupName, targetPath string) error {
	backupPath := filepath.Join(m.BackupDir, backupName)
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("读取备份文件失败: %w", err)
	}
	return os.WriteFile(targetPath, data, 0644)
}
