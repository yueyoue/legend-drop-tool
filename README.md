# 传奇爆率模拟与修改工具

专为传奇私服运营、服务端调试打造的爆率工具，集成**高真实度爆率模拟**、**可视化爆率修改**、**自动备份**三大核心能力。

## 功能特性

### 爆率修改
- 📂 一键加载 MonItems 目录下所有怪物爆率文件
- 🔍 自动识别 HERO/GOM/GEE/BLUE 引擎格式
- ✏️ 单条精准修改概率、掉落数量
- 📦 批量倍率调整、批量设置概率
- ➕ 新增/删除掉落配置
- 🛡️ 修改前自动备份，支持一键还原
- ⚠️ 爆率异常检测（超高/超低/零掉落）

### 爆率模拟
- 🎮 基于传奇原生掉落逻辑的真实模拟
- ⏱️ 可配置模拟时长、击杀比例、刷新频率
- 📊 多维度统计报表（怪物/物品/概率）
- 📝 支持导出模拟结果

### 授权管理
- 🔐 机器码绑定（接口预留，后期实现）
- 🎫 激活码验证（接口预留，后期实现）

## 支持的引擎

| 引擎 | 爆率文件格式 | 状态 |
|------|-------------|------|
| HERO | `1/100 物品名称` | ✅ |
| GOM | `1/100 物品名称` + `#CALL` 引用 | ✅ |
| GEE | `1/100 物品名称` + `#CALL` 引用 | ✅ |
| BLUE | `1/100 物品名称` | ✅ |

## 爆率文件路径

标准路径：`MirServer\Mir200\Envir\MonItems\`

每个怪物一个 `.txt` 文件，文件名为怪物名称，内容格式：
```
; 注释行
1/100 裁决之杖
1/200 圣战戒指 2
#CALL [爆率文件祖玛爆率.txt]
```

## 下载

前往 [Releases](../../releases) 页面下载最新版本的 `legend-drop-tool.exe`。

> 单文件即可运行，中文字体已内嵌，无需额外安装字体或依赖。

## 使用方法

1. 下载 `legend-drop-tool.exe`
2. 双击运行（Windows 10/11 64位）
3. 选择传奇服务端根目录（如 `D:\MirServer`）
4. 点击"加载爆率文件"
5. 在左侧怪物列表中选择要修改的怪物
6. 双击掉落条目进行修改，或使用批量操作

## 编译

需要 Go 1.21+ 和 GCC (CGO for SQLite):

```bash
go mod tidy
go build -ldflags "-s -w -H windowsgui" -o legend-drop-tool.exe ./cmd/
```

> 中文字体（Noto Sans SC）已通过 `//go:embed` 嵌入二进制，编译时自动包含，无需额外步骤。

或通过 GitHub Actions 自动编译（推送 tag 触发）：

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 技术栈

- **语言**: Go 1.21
- **GUI**: Fyne v2
- **架构**: 分层架构（UI/业务逻辑/数据处理）

## 项目结构

```
legend-drop-tool/
├── cmd/
│   └── main.go           # 入口
├── fonts/
│   ├── NotoSansSC.ttf    # 中文字体（编译时嵌入）
│   └── fonts.go          # //go-embed 声明
├── pkg/
│   ├── gui/              # GUI界面
│   │   ├── app.go
│   │   └── theme.go      # CJK中文主题
│   ├── parser/           # 爆率文件解析
│   │   ├── types.go
│   │   └── parser.go
│   ├── editor/           # 爆率编辑
│   │   └── editor.go
│   ├── simulator/        # 爆率模拟
│   │   └── simulator.go
│   ├── backup/           # 备份管理
│   │   └── backup.go
│   ├── auth/             # 授权管理 (预留)
│   │   └── auth.go
│   └── config/           # 配置管理
│       └── config.go
├── .github/workflows/
│   └── build.yml         # CI/CD
└── README.md
```

## License

MIT
