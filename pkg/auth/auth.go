// Package auth 授权管理模块 (接口预留)
// TODO: 后续实现机器码绑定、激活码验证等功能
package auth

// LicenseType 授权类型
type LicenseType int

const (
	LicenseTrial    LicenseType = iota // 试用版
	LicensePermanent                   // 永久授权
	LicenseQuarterly                   // 季度授权
)

func (l LicenseType) String() string {
	switch l {
	case LicenseTrial:
		return "试用版"
	case LicensePermanent:
		return "永久授权"
	case LicenseQuarterly:
		return "季度授权"
	default:
		return "未知"
	}
}

// LicenseInfo 授权信息
type LicenseInfo struct {
	Type       LicenseType
	MachineID  string // 机器码
	Activation string // 激活码
	ExpireAt   int64  // 过期时间戳
	IsActive   bool   // 是否已激活
}

// AuthManager 授权管理接口
type AuthManager interface {
	// GetMachineID 获取本机机器码
	GetMachineID() (string, error)
	// Activate 激活授权
	Activate(activationCode string) (*LicenseInfo, error)
	// CheckLicense 检查授权状态
	CheckLicense() (*LicenseInfo, error)
	// Deactivate 取消激活
	Deactivate() error
}

// LocalAuth 本地授权管理 (基础实现)
type LocalAuth struct {
	info *LicenseInfo
}

// NewLocalAuth 创建本地授权管理
func NewLocalAuth() *LocalAuth {
	return &LocalAuth{
		info: &LicenseInfo{
			Type:     LicenseTrial,
			IsActive: true, // 开发阶段默认激活
		},
	}
}

// GetMachineID 获取本机机器码 (TODO: 实现真实硬件采集)
func (a *LocalAuth) GetMachineID() (string, error) {
	// TODO: 采集 CPU序列号 + 主板序列号 + 硬盘序列号
	// 通过 SHA256 生成唯一机器码
	return "DEV-MACHINE-ID-PLACEHOLDER", nil
}

// Activate 激活授权 (TODO: 实现真实激活逻辑)
func (a *LocalAuth) Activate(activationCode string) (*LicenseInfo, error) {
	// TODO: 验证激活码
	a.info.Activation = activationCode
	a.info.IsActive = true
	a.info.Type = LicensePermanent
	return a.info, nil
}

// CheckLicense 检查授权状态
func (a *LocalAuth) CheckLicense() (*LicenseInfo, error) {
	return a.info, nil
}

// Deactivate 取消激活
func (a *LocalAuth) Deactivate() error {
	a.info.IsActive = false
	a.info.Activation = ""
	return nil
}
