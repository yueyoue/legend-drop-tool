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
- 🎯 指定物品/地图/怪物筛选模拟
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

需要 Go 1.21+ 和 [Wails v2](https://wails.io/) CLI：

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 编译
wails build -platform windows/amd64 -o legend-drop-tool.exe -ldflags "-s -w"
```

或通过 GitHub Actions 自动编译（推送 tag 触发）：

```bash
git tag v2.0.1
git push origin v2.0.1
```

## 技术栈

- **语言**: Go 1.21
- **GUI框架**: [Wails v2](https://wails.io/)（WebView2 前端 + Go 后端）
- **前端**: 原生 HTML/CSS/JS（无框架依赖）
- **架构**: Go 后端绑定 + WebView2 前端渲染
- **CI/CD**: GitHub Actions

## 项目结构

```
legend-drop-tool/
├── main.go                    # Wails 入口
├── wails.json                 # Wails 项目配置
├── internal/
│   └── app/
│       └── app.go             # 后端绑定（所有 public 方法自动暴露给前端）
├── frontend/
│   └── dist/                  # 前端静态资源（嵌入二进制）
│       ├── index.html         # 主页面
│       ├── main.js            # 前端逻辑
│       └── style.css          # 样式
├── pkg/
│   ├── parser/                # 爆率文件解析
│   │   ├── types.go           # 数据类型定义
│   │   └── parser.go          # 解析引擎（支持全部格式）
│   ├── editor/                # 爆率编辑
│   │   └── editor.go
│   ├── simulator/             # 爆率模拟
│   │   └── simulator.go
│   ├── backup/                # 备份管理
│   │   └── backup.go
│   ├── auth/                  # 授权管理（预留）
│   │   └── auth.go
│   └── config/                # 配置管理
│       └── config.go
├── fonts/
│   └── fonts.go               # 字体嵌入
├── .github/workflows/
│   └── build.yml              # CI/CD
└── README.md
```

## License

MIT