package db

// DBType 数据库类型
type DBType string

const (
	DBTypeBDE       DBType = "BDE"
	DBTypeAccess    DBType = "Access"
	DBTypeSQLite    DBType = "SQLite"
	DBTypeMySQL     DBType = "MYSQL"
	DBTypeSQLServer DBType = "SQL Server"
	DBTypeExcel     DBType = "Excel"
)

// ItemInfo 物品信息（从数据库读取）
type ItemInfo struct {
	Idx  int    `json:"idx"`
	Name string `json:"Name"`
}

// DBConfig 数据库连接配置
type DBConfig struct {
	Type     DBType `json:"type"`
	FilePath string `json:"filePath"` // BDE/Access/SQLite/Excel 的文件路径
	Host     string `json:"host"`     // MySQL/SQL Server 地址
	Port     int    `json:"port"`     // MySQL/SQL Server 端口
	DBName   string `json:"dbName"`   // MySQL/SQL Server 数据库名
	User     string `json:"user"`     // MySQL/SQL Server 用户名
	Password string `json:"password"` // MySQL/SQL Server 密码
}

// Reader 数据库读取接口
type Reader interface {
	// ReadItems 读取所有物品列表
	ReadItems() ([]ItemInfo, error)
	// Close 关闭连接
	Close() error
}

// NewReader 根据配置创建数据库读取器
func NewReader(cfg DBConfig) (Reader, error) {
	switch cfg.Type {
	case DBTypeBDE:
		return newODBCReader(cfg, "Paradox")
	case DBTypeAccess:
		return newODBCReader(cfg, "Microsoft Access Driver (*.mdb)")
	case DBTypeSQLite:
		return newSQLiteReader(cfg)
	case DBTypeMySQL:
		return newMySQLReader(cfg)
	case DBTypeSQLServer:
		return newMSSQLReader(cfg)
	case DBTypeExcel:
		return newExcelReader(cfg)
	default:
		return nil, &DBError{Msg: "不支持的数据库类型: " + string(cfg.Type)}
	}
}

// DBError 数据库错误
type DBError struct {
	Msg string
}

func (e *DBError) Error() string {
	return e.Msg
}