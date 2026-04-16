# Kongming 孔明军师系统

<div align="center">

<p>

<h1>🧭 Kongming (孔明)</h1>

<p>

<h3>

运筹帷幄之中，决胜千里之外

</h3>

<p>

<strong>

An Intelligent Multi-Agent Orchestration System

</strong>

</p>

<p>

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://github.com/xfengyin/kongming-agent)
[![License](https://img.shields.io/badge/License-MIT-yellow?style=flat-square)](LICENSE)
[![CI/CD](https://img.shields.io/github/actions/workflow/status/xfengyin/kongming-agent/ci.yml?style=flat-square)](https://github.com/xfengyin/kongming-agent/actions)

</p>

</div>

## 📖 简介

Kongming（孔明）是一个智能多Agent编排系统，灵感来自三国时期蜀汉丞相诸葛亮的智慧。系统通过「军师府」统一调度「五虎将」（子Agent）、「八卦阵」（工作流引擎）、「锦囊库」（技能系统）等核心组件，实现复杂任务的智能分解与协同执行。

## 🏛️ 核心架构

```
┌─────────────────────────────────────────────────────────────┐
│                        军师府 (Commander)                    │
│              运筹帷幄之中，决胜千里之外                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│  │ 五虎将  │  │ 八卦阵  │  │ 锦囊库  │  │ 传令兵  │        │
│  │ Generals│  │ Bagua   │  │  Vault  │  │ Courier │        │
│  └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘        │
│       └────────────┴─────┬──────┴────────────┘            │
│                          │                                 │
│                    ┌─────┴─────┐                           │
│                    │ 参谋部    │                           │
│                    │ Dispatch  │                           │
│                    └─────┬─────┘                           │
│                          │                                 │
├──────────────────────────┼──────────────────────────────────┤
│                    ┌─────┴─────┐                           │
│                    │ 观测台    │                           │
│                    │Observatory│                           │
│                    └───────────┘                           │
└─────────────────────────────────────────────────────────────┘
```

## ✨ 核心特性

| 特性 | 描述 |
|------|------|
| 🔮 **智能编排** | 多Agent协同，自动选择最优策略 |
| 🎯 **八卦阵引擎** | 8种阵法模式（天覆/地载/风扬/云垂/龙飞/虎翼/鸟翔/蛇蟠） |
| 🎁 **锦囊库** | 技能即插即用，热更新支持 |
| ⚔️ **五虎将** | 专业化Agent池，智能调度 |
| 📊 **可观测性** | Prometheus + OpenTelemetry 全链路追踪 |
| 🔄 **容错重试** | 智能重试 + 熔断器机制 |
| 🚀 **高性能** | 并发执行，优雅退出 |

## 🚀 快速开始

### 安装

```bash
# 克隆项目
git clone https://github.com/xfengyin/kongming-agent.git
cd kongming-agent

# 下载依赖
go mod download

# 构建
make build

# 运行
./kongming
```

### 运行示例

```bash
# 运行快速开始示例
go run ./examples/quickstart/main.go
```

## 📦 项目结构

```
kongming/
├── cmd/                          # 应用入口
│   └── kongming/
│       └── main.go              # 主程序
├── configs/                     # 配置文件
│   └── kongming.yaml            # 主配置
├── internal/                    # 内部包
│   └── memory/                  # 记忆系统
├── pkg/                         # 核心包
│   ├── bagua/                   # 八卦阵引擎
│   ├── cmd_center/              # 军师府/参谋部
│   ├── courier/                 # 传令兵
│   ├── dispatch/                # 调度器
│   ├── generals/                # 五虎将池
│   ├── observatory/             # 观测台
│   ├── repeater/                # 复读机/熔断器
│   └── strategy_vault/          # 锦囊库
├── examples/                    # 示例
│   ├── quickstart/             # 快速开始
│   ├── longzhong_strategy/     # 隆中对策略
│   ├── wuhu_campaign/          # 五虎北伐
│   └── zhuge_bagua/            # 诸葛八卦
├── deployments/                 # 部署配置
│   ├── prometheus/
│   └── grafana/
├── .github/workflows/           # CI/CD
├── Makefile
└── README.md
```

## 🛠️ 开发

```bash
# 格式化代码
make fmt

# 运行测试
make test

# 代码检查
make lint

# 全部检查
make ci

# 运行示例
make run-example
```

## 📊 观测

启动后访问以下端点：

| 端点 | 描述 |
|------|------|
| `:9090/metrics` | Prometheus 指标 |
| `:9090/health` | 健康检查 |
| `:9090/ready` | 就绪检查 |

## 🐳 Docker部署

```bash
# 构建镜像
make docker-build

# 运行容器
make docker-run

# 使用 docker-compose
docker-compose up -d
```

## 📚 相关资源

- [设计文档](./docs/)
- [API 文档](./docs/api.md)
- [架构说明](./docs/architecture.md)

## 🤝 贡献

欢迎提交 Issue 和 Pull Request！

## 📄 许可证

MIT License - 详见 [LICENSE](LICENSE)

---

<div align="center">

<p>

<strong>

「非淡泊无以明志，非宁静无以致远」

</strong>

<p>

诸葛孔明

</p>

</div>
