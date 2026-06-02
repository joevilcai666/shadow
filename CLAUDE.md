# Shadow — AI 工程师作战手册

> 本文件是每个会话的**最高优先级指令**。所有规则必须无条件遵守。
> 如有疑问，先问用户，不要猜测。

---

## 1. 项目概述与整体架构

### 1.1 项目基本信息

| 项目          | 值                                      |
|---------------|-----------------------------------------|
| GitHub Repo   | `https://github.com/joevilcai666/shadow` |
| 产品定位      | 给 coding agent 用的「记忆层」(记忆插件 / 工具) |
| 核心价值      | 你纠正 Agent 一次，所有你用的 agent 都记住 |
| 一期 PMF      | 为重度使用「coding agent」的「个人开发者」，提供把每一次「纠正」变成 agent 永不再犯的规则的服务 |
| AgentOS Board | `/Users/jichuncai/thinking-vault/01_projects/Shadow/agent-os/board.md` |
| PRDs 文档     | `/Users/jichuncai/thinking-vault/01_projects/Shadow/shadow-PRDs` |
| 架构形态      | 常驻本地 daemon + CLI TUI + localhost web 管理界面 |

### 1.2 四大核心能力

```
┌─────────────────────────────────────────────────────────────┐
│  Shadow 记忆层                                              │
├─────────────┬─────────────┬─────────────┬─────────────────┤
│ ① 自动捕获   │ ② 归属存储   │ ③ 翻译官     │ ④ 万能接口       │
│ (行车记录仪)  │ (云盘)       │ (护城河)      │ (USB-C)          │
│ 默默记下每次  │ 数据归你、跨  │ 把杂乱信号    │ 插进任何模型/     │
│ 纠正、不用手动│ 设备同步、   │ 提炼成规则    │ 工具/机器人能用   │
│ 保存          │ 能带着走      │ 再翻成每个    │                  │
│               │             │ 工具的母语    │                  │
└─────────────┴─────────────┴─────────────┴─────────────────┘
```

### 1.3 架构模式：Daemon + CLI + Web 三层

```
┌─────────────────────────────────────────────────────────────┐
│  localhost web (可选)                                       │
│  管理界面 / 规则审阅 / Aha demo / 云同步登录                  │
└──────────────────────┬──────────────────────────────────────┘
                       │ HTTP (localhost:7878)
┌──────────────────────▼──────────────────────────────────────┐
│  CLI TUI (`shadow start`)                                   │
│  安装引导 / 授权范围 / agent 检测 / 初始记忆生成               │
└──────────────────────┬──────────────────────────────────────┘
                       │ IPC
┌──────────────────────▼──────────────────────────────────────┐
│  Local Daemon (后台服务)                                    │
│  文件监听 / 纠正捕获 / 规则结晶 / 上下文写入 / 跨工具同步       │
└──────────────────────┬──────────────────────────────────────┘
                       │ 读写
┌──────────────────────▼──────────────────────────────────────┐
│  Target Agent 原生上下文                                     │
│  CLAUDE.md / .cursorrules / .github/copilot-instructions.md  │
└─────────────────────────────────────────────────────────────┘
```

### 1.4 目标集成（一期）

| Agent | 目标文件 | 状态 |
|-------|---------|------|
| Claude Code | `CLAUDE.md` / `.claude/` | 规划中 |
| Cursor | `.cursorrules` | 规划中 |
| GitHub Copilot | `.github/copilot-instructions.md` | 规划中 |
| Codex / 其他 | 待扩展 | 规划中 |

### 1.5 目录结构

```
Shadow/
├── shadow-PRDs/          — 产品需求文档
│   ├── Shadow_Product_Strategy.md
│   ├── Onboarding 设计需求.md
│   └── 记忆可视化与管理界面设计需求.md
├── agent-os/             — 多 Agent 工程执行系统
│   ├── board.md         — 状态看板
│   ├── shared-context.md — 共享上下文
│   ├── progress-update.md — 进度快照
│   ├── agents/          — Agent 注册表
│   ├── tickets/         — 任务卡片
│   ├── blockers/        — 阻塞卡
│   └── runs/            — 执行日志
└── (源码目录 TBD)        — Shadow daemon / CLI / web 实现
```

### 1.6 核心命令（待实现，具体待定）

```bash
# 安装 (待实现)
brew install shadow  # 或 npm install -g shadow

# 启动 daemon + CLI 向导
shadow start

# 打开管理界面
shadow open

# 查看状态
shadow status

# 手动触发同步
shadow sync
```

---

## 2. 核心业务规则清单

### 2.1 核心价值链：「纠正 → 规则」闭环

- 用户在 coding agent 输出上做纠正（接受/拒绝/修改）
- Shadow 自动捕获该纠正事件
- 「翻译官」模块将纠正结晶为结构化规则
- 规则自动写入目标 agent 的原生上下文文件
- 跨工具生效：在 Claude Code 学到的东西，Cursor 也能用

### 2.2 Onboarding 流程（渐进式双层）

**第 1 层 · CLI（必经，~60s）**
- 安装 → daemon 注册 → 授权范围 → 自动检测并接入 agent → 后台静默扫仓
- 命令行内闭环，零打断、不强制开浏览器

**第 2 层 · localhost web（可选但默认引导）**
- 扫仓结果审阅、已有规则导入归一化、Aha demo 对比、可选登录
- CLI 完成核心接入后提示 `✓ Shadow 已就绪 — 按 Enter 在浏览器查看你的初始记忆`

**端到端生命周期**
```
brew/npm 安装 → shadow start (daemon 注册) → 授权范围 →
自动检测 agent → 一键接入 → 后台静默扫仓 → (Enter 打开 web)
→ Aha demo 对比 → 日常 coding · 无感捕获
```

### 2.3 数据归属原则

- **本地优先**：数据默认只存本地，全程可跳过、可回滚
- **纯本地用户全程零登录**：登录只在主动点「云同步 / 升级 Pro」时出现
- **不卖用户数据**：和用户的根本承诺，不能自相矛盾

### 2.4 三条业务线并进

| 业务线 | 做什么 | 角色 |
|--------|--------|------|
| 开源 | Shadow 本地插件 | 在开发者群体里增加品牌曝光、提升信任度 |
| C 端 | 共鸣社交 App (背后也是 Shadow 技术) | 数据入口 |
| B 端 | Shadow 云端服务; 企业定制化 | 早期营收最大的入口 |

### 2.5 一期 PMF 验证指标

- 「纠正 → 规则」闭环跑通，跨工具生效
- 用户体感「真牛逼」+ 愿意付费

---

## 3. 编码规范与最佳实践

### 3.1 技术选型（待确认）

- 语言：待定（可能 Go/Rust 用于 daemon，TypeScript 用于 web）
- 包管理：待定
- 架构：待定

> 注意：Shadow 一期 MVP 尚未拆票，技术栈待定。以上仅为业务层规范。

### 3.2 文件命名规范

- 文件名：PascalCase 或 kebab-case 视上下文而定
- 类型名：PascalCase
- 函数/变量：camelCase
- 配置/常量：SCREAMING_SNAKE_CASE

### 3.3 代码风格

- 使用 `// MARK: -` 分段组织代码
- 遵循现有文件的缩进和格式
- 只写必要的注释（WHY，不是 WHAT）
- import 顺序：系统框架 → 第三方库 → 项目内模块

---

## 4. AgentOS 工作流

### 4.1 状态流

`backlog -> ready -> assigned -> in_progress -> testing -> done`

阻塞路径：
`in_progress -> blocked -> in_progress/testing`

### 4.2 AgentOS 命令

```bash
# 更新进度
05_system/scripts/agent-os/update-progress.sh

# 领取任务
05_system/scripts/agent-os/assign.sh --max-agents 5

# 开始任务
05_system/scripts/agent-os/start.sh --ticket <id> --agent <id>

# 心跳
05_system/scripts/agent-os/heartbeat.sh --ticket <id> --agent <id> --status <in_progress|blocked|testing> --note "..."

# 完成任务
05_system/scripts/agent-os/complete.sh --ticket <id> --pr <url> --commit <sha> --test <pass|fail>

# 调和新阻塞
05_system/scripts/agent-os/reconcile.sh

# 刷新看板
05_system/scripts/agent-os/board.sh
```

### 4.3 Agent 优先级政策

`P0 > P1 > P2 > P3`

### 4.4 Agent 注册表

当前活跃 Agent：`agent-core-01`, `agent-int-01`, `agent-fe-01`, `agent-infra-01`, `agent-qa-01`

---

## 5. 已知痛点 & 历史决策记录

### 5.1 一期规划模块

| Module | Description | Priority |
|--------|-------------|----------|
| Capture Engine | 自动捕获用户对 coding agent 的纠正 | P0 |
| Rule Crystallization | "翻译官"：纠正 → 结构化规则 | P0 |
| Tool Adapters | 写入各 agent 原生上下文 (CLAUDE.md, .cursorrules, etc.) | P0 |
| Local Daemon | 常驻本地服务 + 文件监听 | P0 |
| Management UI | 查看/编辑/删除规则的管理界面 | P1 |
| Cross-tool Sync | 跨工具记忆同步 | P1 |
| Cloud Sync | 跨设备云同步 | P2 |

### 5.2 关键决策记录

- **从 coding agent 切入**：先在「最痛的地方」跑通。程序员同时用 Cursor、Claude Code、Copilot，记忆不通、反复教同样的东西给不同 agent
- **本地优先**：数据默认只存本地，全程可跳过、可回滚，降低用户戒心
- **渐进式双层 onboarding**：CLI 负责「快」（60s 闭环），localhost web 负责「重」（审阅、可视化、对比）
- **翻译官是护城河**：把杂乱信号提炼成规则，再翻成每个工具的「母语」

---

## 6. 标准工作流（Plan → Act → Reflect）

> 每次任务必须严格按以下 5 步执行，不得跳步。

### Step 1: 理解需求，输出 Plan

- **永远先输出 Plan，不要直接写代码**
- 明确列出：要改哪些文件、为什么要改、预期效果
- 如果需求有歧义，列出所有可能的理解，让用户选
- 如果有更简单的方案，直接说出来让用户决定
- 复杂变更（影响 3+ 文件）用 Plan Mode

### Step 2: 确认测试范围

- 确认现有测试不会被破坏
- 新功能先写行为契约测试
- Bug fix 先写 failing test 复现 bug

### Step 3: 实现修改

- **精准手术式修改**：只改必须改的，不顺手"改进"旁边代码
- 匹配现有代码风格，即使自己有不同偏好
- 删除自己引入的无用 import / 变量 / 函数
- 每行改动都能追溯到用户的具体需求

### Step 4: 验证

- 跑测试（如有）
- UI 改动：构建 → 截图 → Claude Vision 检查
- 验证通过后才算完成

### Step 5: Reflect & 收尾

- 检查是否有遗漏的 edge case
- 确认没有引入新的 warning 或 error
- 更新 AgentOS Board ticket 状态
- 上下文接近满时执行 `/compact`

### 异常处理规则

- **3 次 fix 失败就停下问**：不要反复尝试，把当前状态、已尝试的方案、卡住的原因列出来让用户决定
- **卡住超过 3 轮就停下来问**
- **不确定时猜测——先问用户**

---

## 7. 额外技巧与提示

### 7.1 多模型分工策略

| 任务类型 | 推荐做法 |
|---------|---------|
| 架构设计 / 复杂 Plan | Opus（深度思考） |
| 代码实现 / Bug 修复 | Sonnet（快速精准） |
| 视觉检查 / UI 验证 | Claude Vision（截图分析） |

### 7.2 卡住时的处理策略

1. **第 1 次失败**：重新阅读错误信息，检查拼写和逻辑
2. **第 2 次失败**：搜索代码库中类似模式，看其他地方怎么做的
3. **第 3 次失败**：**停下来**，向用户报告：
   - 当前状态和已尝试的方案
   - 怀疑的根因
   - 下一步建议
4. **绝对禁止**盲目尝试第 4 次

### 7.3 Prompt 模板（给用户的建议）

```
# 修 bug 模板
[文件路径] 的 [功能] 出现 [错误现象]
错误信息：[贴错误日志]
我期望的行为是：[描述]

# 加功能模板
在 [模块] 添加 [功能]
需求：[详细描述]
设计参考：[如有 Figma 链接请附上]
```

### 7.4 Compact Instructions

当系统触发 compact 或 `/compact` 时，**必须保留**（按优先级）：
1. 核心命令（1.6 节）
2. 标准工作流（第 6 节）
3. 铁律（第 8 节 NEVER & ALWAYS）
4. AgentOS 命令（4.2 节）
5. 一期规划模块（5.1 节）
6. AgentOS Board 路径
7. 当前 Ticket 编号（如果存在）

---

## 8. 铁律（NEVER & ALWAYS）— 必须 100% 遵守

### NEVER（绝对禁止）

- 不确定时猜测——先问用户
- 添加用户没要求的功能、抽象、"以防万一"的代码
- 空 `catch {}` 或静默吞掉错误
- 在 CLAUDE.md 里写详细测试脚本（走 Skill）

### ALWAYS（强制执行）

- 任何修改前先确认测试范围
- 永远先输出 Plan，不要直接写代码
- Bug 修复先写 failing test
- Commit Message 严格遵循 Commit-Standard Skill
- 3 次 fix 失败就停下问 + 重新分析
- 每行改动都能追溯到用户的具体需求
- 上下文接近满时执行 `/compact`
- 保持 CLAUDE.md 与实际项目状态同步

# >>> shadow managed >>>
# [Shadow auto-managed rules — do not edit between markers]
# Always output Plan first before writing code. Bug fix requires failing test first.
# This project uses Go for the daemon/CLI, with React+TypeScript for the web UI
# Package manager: Go modules (go.mod detected)
# <<< shadow managed <<<
