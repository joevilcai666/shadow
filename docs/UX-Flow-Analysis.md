# Shadow 用户体验流程分析 & PRD 差距报告

> **文档定位**：从资深 full-stack engineer 和 HCI 用户体验专家视角，对 Shadow MVP 当前代码实现与 PRD 需求文档进行完整深度比对。覆盖用户每一步操作、整体体验评估、差距分析、和优化建议。
>
> **分析日期**：2026-06-02
> **代码版本**：v0.1.0 (commit 45a682a)

---

## 目录

1. [总体评估摘要](#1-总体评估摘要)
2. [用户完整生命周期体验流程](#2-用户完整生命周期体验流程)
3. [Onboarding 逐屏体验分析（核心重点）](#3-onboarding-逐屏体验分析核心重点)
4. [日常使用体验分析](#4-日常使用体验分析)
5. [Web 管理界面体验分析](#5-web-管理界面体验分析)
6. [PRD 需求覆盖度矩阵](#6-prd-需求覆盖度矩阵)
7. [关键差距与优先修复建议](#7-关键差距与优先修复建议)
8. [Onboarding 优化方案](#8-onboarding-优化方案)

---

## 1. 总体评估摘要

### 完成度概览

| 模块 | PRD 要求 | 实现状态 | 完成度 |
|------|---------|---------|--------|
| CLI 入口与命令 | 7 个命令 | 6 个已实现（缺 `review`） | 🟡 85% |
| Daemon 后台服务 | 常驻运行、IPC、状态管理 | 完整实现 | 🟢 95% |
| Onboarding TUI | 4 步引导向导 | 代码存在但 **未接入** `startCmd` | 🔴 20% |
| 捕获引擎 | 多通道、多信号类型 | 仅 Claude Code 日志 + 否定/标记模式 | 🟡 40% |
| 提炼/结晶引擎 | LLM + 规则回退、相似度、冲突 | 引擎已实现但 **未接入实时管线** | 🟡 50% |
| 存储层 | 完整数据模型 + 版本历史 | SQLite + 5 个 repo，基本完整 | 🟢 90% |
| 适配器 | Claude Code / Cursor / Codex / Copilot | 3 个适配器 + managed block | 🟡 75% |
| Web 管理界面 | 8 个页面（看板、规则、详情、审核、冲突、Agent、设置） | 4 个基础页面，无详情/审核/冲突 | 🟡 35% |
| Web Onboarding 向导 | 审阅初始记忆 + Aha Demo | **完全未实现** | 🔴 0% |
| 隐私护栏 | 密钥不入库、只存提炼规则、排除模式 | 已实现，regex 检测 + 配置排除 | 🟢 85% |

### 总体评价

**架构基础扎实，但用户体验链路存在多处断裂。** 核心问题是：
1. **Onboarding 是假活**：TUI 向导代码已写好，但 `shadow start` 没有调用它，用户实际看到的是纯文本输出
2. **管线未打通**：捕获 → 提炼 → 写入的闭环在 daemon 中没有自动连接，信号入库了但没有后续自动结晶
3. **Web 界面是骨架**：UI 组件存在但大量硬编码，缺少关键页面（规则详情、审核队列、冲突解决）
4. **数据流断裂**：数据库有完善的数据模型，但前端展示不充分（无时间线溯源、无版本 diff）

---

## 2. 用户完整生命周期体验流程

### 理想流程（PRD 定义）

```
brew/npm 安装 → shadow start (daemon 注册) → 授权范围 →
自动检测 agent → 一键接入 → 后台静默扫仓 → (Enter 打开 web)
→ Web 审阅初始记忆 → Aha demo 对比 → 日常 coding · 无感捕获
```

### 当前实际流程（用户视角）

以下描述用户在当前代码下 **实际** 会经历的每一步：

---

### Step 0：安装

```
$ brew install shadow        # ← 尚未发布到 Homebrew
  # 或
$ npm install -g shadow      # ← 尚未发布到 npm
  # 实际方式：
$ go build -o shadow .       # 从源码编译
```

**用户体验评估**：
- ❌ 无预编译 binary 分发（GoReleaser 配置存在但未实际发布）
- ❌ 无 Homebrew formula（Formula/ 目录存在但未完善）
- ⚠️ 只有 `go build` 一种方式，门槛偏高
- **PRD 要求**：`brew install shadow` 或 `npm i -g shadow` 一行安装

**用户感受**：*"这是个内部工具还是产品？我还要自己编译？"*

---

### Step 1：`shadow start`

```bash
$ shadow start

# 当前实际输出（main.go:56-82）：
✓ Shadow daemon registered with launchd
Starting daemon...
✓ Shadow daemon started

Shadow v0.1.0 — ready!
```

**用户体验评估**：
- ❌ **没有 TUI 向导**：代码里有完整的 `OnboardingModel`（onboarding.go），但 `startCmd` 注释了 `// TODO: SHADOW-013 — onboarding TUI`
- ❌ 没有品牌 Header、没有步骤引导、没有 "共 4 步约 60 秒" 的提示
- ❌ 没有授权范围确认
- ❌ 没有 Agent 检测与选择
- ❌ 没有扫仓和初始记忆生成
- ❌ 没有浏览器打开提示
- ✅ launchd 注册正常工作
- ✅ daemon 正常启动

**PRD 要求 vs 实际**：

| PRD 要求 | 实际 |
|---------|------|
| 品牌 Header（👻 Shadow v0.1.0 · MVP） | 无 |
| "你纠正一次，所有本地 coding agent 都记住。" | 无 |
| "即将完成 4 步（约 60 秒）" | 无 |
| Enter 开始 / q 退出 | 直接执行，无交互 |
| 检测到 daemon 已运行 → 跳到已就绪面板 | ✅ 有基本检测 |

**用户感受**：*"装完了？然后呢？我怎么知道它在工作？"*

---

### Step 2：授权范围（PRD Step 2/4）

**当前状态**：❌ **完全缺失**

用户在 `shadow start` 后不会看到任何授权提示。PRD 要求的界面：

```
┌─ Step 2/4 · 授权访问范围 ──────────────┐
│  Shadow 需要读写以下内容才能工作：       │
│   [✓] 读取项目代码与会话日志            │
│   [✓] 写入各 agent 原生上下文文件       │
│  🔒 默认保护（已为你开启）：            │
│   • 绝不入库：密钥 / token / 凭证       │
│   • 排除目录：node_modules, .git, .env* │
│   ▸ Enter 接受推荐设置                  │
└─────────────────────────────────────────┘
```

**实际发生**：daemon 直接开始运行，默认配置生效（config.go 的 DefaultConfig），但用户**不知道**也**无法确认**。

**用户感受**：*"它在读我的什么文件？我有选择吗？"*

---

### Step 3：Agent 检测与接入（PRD Step 3/4）

**当前状态**：⚠️ TUI 代码存在但未调用

onboarding.go 的 `detectAgents()` 仅检测 3 个 agent：
- Claude Code：检查 `~/.claude` 目录存在
- Cursor：检查 `/Applications/Cursor.app` 存在
- GitHub Copilot：检查 `gh` 命令在 PATH 中

**缺失功能**：
- ❌ 没有显示每个 agent 的写入目标文件路径（如 "→ 写 CLAUDE.md"）
- ❌ 没有接入粒度选择（全局 + 当前项目 / 仅当前项目）
- ❌ 没有 Codex 检测（PRD 列出了 4 个 agent）
- ❌ 没有逐个显示接入结果 `✓ Claude Code 已接入`
- ❌ 没有安全合并提示（已有手写文件时）

**用户感受**：*"它支持什么 agent？我选了之后会发生什么？"*

---

### Step 4：初始记忆生成（PRD Step 4/4）

**当前状态**：⚠️ Scanner 和 Importer 代码存在但未接入

onboarding.go 的 `scanProject()` 只统计文件存在数量，**不实际生成规则**：

```go
// 当前实现：只计数
for _, f := range files {
    if _, err := os.Stat(f); err == nil {
        count++
    }
}
return scanCompleteMsg{count: count}
```

**实际存在的但未接入的代码**：
- `capture/scanner.go` 的 `Scanner` 和 `Importer` 可以真正扫描和导入
- `scanner.go` 的 `ToRules()` 可以将扫描结果转为候选规则
- `Importer.ImportFile()` 可以读取 CLAUDE.md/.cursorrules 并归一化

**缺失功能**：
- ❌ 扫仓结果不展示给用户（包管理器、测试框架等发现）
- ❌ 不导入已有规则文件（CLAUDE.md / .cursorrules）
- ❌ 没有轻问卷（2 个关键问题）
- ❌ 不生成候选规则到数据库
- ❌ 没有写入 agent 上下文文件

**用户感受**：*"扫仓扫了什么？生成了什么？我怎么什么都看不到？"*

---

### Step 5：完成与移交 Web（PRD Step 5/4）

**当前状态**：❌ **缺失关键交互**

onboarding.go 的完成屏存在基本框架，但：
- ❌ 没有 "按 Enter 打开浏览器" 功能
- ❌ 没有 `http://localhost:7878/welcome` 的 welcome 路由
- ❌ 没有显示可量化的战果（接入数 / 生成规则数 / 待审数）
- ✅ 列出了常用命令（status / open / review）

**用户感受**：*"然后呢？现在要干嘛？"*

---

### Step 6：Web 首次接入向导（PRD Section 3.6）

**当前状态**：❌ **完全未实现**

PRD 定义了三步 web 向导：
1. **审阅初始记忆**：按全局/项目分组，每条显示来源 + 置信度，可勾选启用
2. **Aha demo 对比**：并排展示 "有/无 Shadow" 的 agent 行为差异
3. **完成**：进入控制台主界面

当前 web 端：
- 无 `/welcome` 路由
- 无任何向导流程
- 打开 `http://localhost:7878` 直接进入 Dashboard（且数据为空）

**用户感受**：*"这界面什么都没有，它真的在工作吗？"*

---

### Step 7：日常使用 · 无感捕获

**当前状态**：⚠️ 部分工作

**工作正常的部分**：
- ✅ Daemon 后台常驻运行（launchd KeepAlive=true）
- ✅ Claude Code 日志监听（fsnotify）
- ✅ 否定模式识别（中英文关键词匹配）
- ✅ 手动标记识别（"记住" "remember" 等）
- ✅ 隐私过滤（敏感数据不入库）
- ✅ 信号存储到 sources 表

**不工作的部分**：
- ❌ 捕获到的信号**不会自动结晶为规则**（distill engine 未接入实时管线）
- ❌ 没有 manual_edit 信号类型（手改 agent 产出）
- ❌ 没有 git_revert 信号类型
- ❌ 没有 repetition 信号检测
- ❌ 没有 Cursor 和 Copilot 的日志解析器
- ❌ MCP server 存在但未主动捕获信号

**用户感受**：*"我纠正了好几次，Shadow 怎么没记住？" ← 这是最致命的体验断层*

---

## 3. Onboarding 逐屏体验分析（核心重点）

Onboarding 是用户对产品的**第一印象**，PRD 将其定义为 "60 秒完成、装完即用、零打断"。以下是每个环节的详细体验分析：

### 3.1 CLI · 启动总览

| 维度 | PRD 定义 | 当前实现 | 差距 |
|------|---------|---------|------|
| 品牌 Header | 👻 Shadow + 版本 + 一句话价值 | 无 Header | 🔴 |
| 步骤预告 | "即将完成 4 步（约 60 秒）" | 无 | 🔴 |
| 隐私承诺 | "你的数据默认只存本地" | 无 | 🔴 |
| 开始交互 | Enter 开始 / q 退出 | 无交互，直接执行 | 🔴 |
| 加载态 | Spinner "正在注册后台服务…" | 纯文本 "Starting daemon..." | 🟡 |
| 错误处理 | 端口被占用 → 修复指引 | 基本错误信息 | 🟡 |
| 已运行检测 | 跳过注册，进入已就绪面板 | ✅ 检测并打印状态 | 🟢 |

**体验评分**：2/10 — 缺少最重要的品牌展示和交互引导

### 3.2 CLI · 授权与隐私范围

| 维度 | PRD 定义 | 当前实现 | 差距 |
|------|---------|---------|------|
| 授权勾选 | 可勾选的权限列表 | 无（onboarding.go step 2 是 agent 选择，不是授权） | 🔴 |
| 隐私护栏展示 | 显式可见的隐私承诺 | 无 | 🔴 |
| 高级配置 | 'a' 键展开高级设置 | 无 | 🔴 |
| 敏感目录排除 | 预置 + 可编辑 | 无 | 🔴 |
| 取消写入警告 | 取消写入授权 → 二次确认 | 无 | 🔴 |

**体验评分**：0/10 — 此步骤在当前流程中完全缺失

### 3.3 CLI · Agent 检测与接入

| 维度 | PRD 定义 | 当前实现 | 差距 |
|------|---------|---------|------|
| 自动检测 | 4+ agent | 3 agent（无 Codex） | 🟡 |
| 写入目标展示 | "→ 写 CLAUDE.md (项目)" | 有 description 但不精确 | 🟡 |
| 默认全选 | ✅ | ✅ | 🟢 |
| 接入粒度 | 全局+项目 / 仅项目 | 无选择 | 🔴 |
| 接入结果 | 逐个显示 ✓ / ✗ | 无（检测后直接跳到 step 4） | 🔴 |
| 安全合并 | 已有手写文件 → 安全合并提示 | 无 | 🔴 |

**体验评分**：3/10 — 基本检测逻辑存在，但无实际接入操作

### 3.4 CLI · 初始记忆生成

| 维度 | PRD 定义 | 当前实现 | 差距 |
|------|---------|---------|------|
| 扫仓推断 | 包管理器/测试框架/目录约定 | Scanner 代码存在但未调用 | 🔴 |
| 导入归一化 | CLAUDE.md/.cursorrules → Shadow 规则 | Importer 代码存在但未调用 | 🔴 |
| 轻问卷 | 2 个关键问题，可跳过 | 无 | 🔴 |
| 生成草稿 | 候选规则写入数据库 | 无 | 🔴 |
| 写入 agent | managed block 写入原生上下文 | 无 | 🔴 |
| 空仓处理 | 不报错，提示"边用边积累" | 无 | 🔴 |

**体验评分**：1/10 — 核心价值（"你已经有了初始记忆"）完全缺失

### 3.5 CLI · 完成与移交 Web

| 维度 | PRD 定义 | 当前实现 | 差距 |
|------|---------|---------|------|
| 战果展示 | "3 个 agent 已接入 · 生成 9 条初始记忆" | 框架存在但数据为空 | 🟡 |
| 浏览器打开 | Enter 打开 localhost:7878/welcome | 无浏览器打开 | 🔴 |
| CLI 命令列表 | status / review / open | ✅ 有 | 🟢 |
| 纯终端路径 | s 跳过，留在终端 | 无选择 | 🔴 |

**体验评分**：3/10 — 基本框架存在，缺少关键交互

### 3.6 Web · 首次接入向导

**体验评分**：0/10 — 完全未实现

### 3.7 Web · Aha Demo 对比

**体验评分**：0/10 — 完全未实现

---

## 4. 日常使用体验分析

### 4.1 捕获体验

**用户视角流程**：
1. 用户在 Claude Code 中纠正 agent："不对，这个项目用 pnpm"
2. Claude Code 写入 JSONL 会话日志
3. Shadow daemon 的 fsnotify 监测到文件变化
4. ClaudeCodeParser 解析新内容
5. ClassifySignal 匹配到否定模式 → Signal{Type: "explicit_instruction", Strength: "strong", Confidence: 0.85}
6. 隐私过滤通过
7. 存入 sources 表
8. ❌ **到此为止** — 没有后续自动结晶

**用户看到的**：什么都没发生。没有通知，没有 badge，没有新规则。

**PRD 要求**：信号应在累积到阈值后自动结晶为候选规则，并提示用户审核。

### 4.2 提炼/结晶体验

**当前管线断裂点**：
- `daemon.go` 创建了 `distillEngine` 但没有启动任何定时/事件驱动的蒸馏循环
- 信号存在 `sources` 表里但没有 `rule_id`（未关联到规则）
- LLM distiller 和 rule-based distiller 都有完整实现，但从未被调用

**PRD 要求**：显式信号 → 立即升为候选；隐式信号 → 达阈值后升为候选。

### 4.3 跨工具生效体验

**当前状态**：
- ✅ 适配器代码完整（ClaudeCodeAdapter、CursorAdapter）
- ✅ ManagedBlock 有安全写入（标记块 + 备份 + MD5 校验）
- ❌ 没有自动触发写入（无规则变更 → 适配器写入的监听）
- ❌ 没有 Codex 适配器的实际写入逻辑（codex.go 是存根）
- ❌ 没有 GitHub Copilot 适配器

**用户视角**：即使在数据库里手动创建规则，也不会自动写入 agent 上下文文件。

### 4.4 CLI 日常命令

| 命令 | PRD | 实现 | 状态 |
|------|-----|------|------|
| `shadow start` | 启动 daemon + onboarding | 仅启动 daemon | 🟡 |
| `shadow status` | 查看接入与生效状态 | ✅ 可用，返回 JSON 状态 | 🟢 |
| `shadow open` | 打开 web 控制台 | ✅ 打开浏览器 | 🟢 |
| `shadow stop` | 停止 daemon | ✅ 可用 | 🟢 |
| `shadow review` | 在终端审阅候选规则 | ❌ 未实现 | 🔴 |
| `shadow rules` | 管理规则 | ❌ 未实现 | 🔴 |
| `shadow uninstall` | 卸载并可选回滚 | ✅ 基本可用，`--clean-blocks` 未实现 | 🟡 |
| `shadow version` | 显示版本 | ✅ | 🟢 |
| `shadow serve` | 前台运行 daemon | ✅ 开发用 | 🟢 |

---

## 5. Web 管理界面体验分析

### 当前页面结构

```
localhost:7878/
├── / (Dashboard)     — 基础统计卡片
├── /rules            — 规则列表 + 搜索/筛选
├── /projects         — 项目列表
└── /settings         — 设置（全硬编码）
```

### PRD 要求 vs 实际

| PRD 页面 | PRD 功能 | 当前状态 |
|---------|---------|---------|
| **全局框架** | 左边栏 + 项目切换器 + 搜索 + 通知 + 明暗切换 | 左边栏有，缺项目切换器/搜索/通知/主题切换 |
| **Dashboard 看板** | 复发率趋势、本周避免错误、各 agent 生效次数、采纳率漏斗、记忆健康度 | 4 个基础数字卡片，agents 数量硬编码=2，无图表 |
| **规则列表** | 表格/卡片切换、多维筛选（分类/状态/来源/agent）、排序、全局/项目分组 | 搜索 + 状态筛选，无表格/卡片切换、无分组 |
| **规则详情 + 时间线** | 完整字段、溯源时间线、内联编辑、版本 diff + 回滚 | ❌ 完全缺失 |
| **待生效审核队列** | 按置信度分组、单条/批量通过/驳回、来源解释 | ❌ 完全缺失 |
| **冲突解决** | 并排面板、系统建议、4 种裁决方式 | ❌ 完全缺失 |
| **Agent 接入** | 每 agent 状态/写入目标/同步时间/命中数 | Settings 里有硬编码状态，无独立页面 |
| **设置** | 全局/项目开关（真实）、隐私仪表盘、账户、关于 | 全部硬编码展示，无真实交互 |

### Dashboard 具体问题

```typescript
// Dashboard.tsx 第 29 行：agents 数量硬编码
{ label: 'Agents', value: 2, icon: Zap, color: 'text-purple-400' },
```

```typescript
// Dashboard.tsx 第 52 行：Connected Agents 列表硬编码
{['Claude Code', 'Cursor'].map(agent => (
```

**影响**：用户看到的信息与实际状态可能不符，破坏信任。

### Rules 页面具体问题

- ✅ 有搜索功能
- ✅ 有状态筛选（All / Active / Candidate / Disabled）
- ✅ 有批量操作（Activate / Disable / Delete）
- ✅ 有单条删除和状态切换
- ❌ 缺少 "conflicted" 状态筛选
- ❌ 缺少分类筛选
- ❌ 缺少来源筛选
- ❌ 缺少项目分组
- ❌ 缺少表格/卡片视图切换
- ❌ 缺少排序选项
- ❌ 缺少规则详情展开/抽屉
- ❌ 缺少置信度条可视化
- ❌ 缺少来源链接

### Settings 页面具体问题

**所有值都是硬编码**，没有连接到真实 API：

```typescript
// Settings.tsx：没有 API 调用，全部静态数据
settings: [
  { label: 'Capture Enabled', value: 'Yes', type: 'toggle' },  // 硬编码
  { label: 'Claude Code', value: 'Connected', type: 'status' }, // 硬编码
```

**影响**：用户修改设置不会生效，开关不能点击。

---

## 6. PRD 需求覆盖度矩阵

### P0 需求（MVP 必须完成）

| # | 需求 | 来源 | 实现状态 | 影响等级 |
|---|------|------|---------|---------|
| 1 | CLI 安装 + daemon 后台注册 | PRD §13.1 | ✅ 基本完成 | - |
| 2 | Onboarding TUI 4 步引导 | Onboarding PRD §3.1-3.5 | 🔴 代码存在但未调用 | **P0 致命** |
| 3 | 授权范围确认 | Onboarding PRD §3.2 | 🔴 缺失 | **P0 严重** |
| 4 | Agent 自动检测与接入 | Onboarding PRD §3.3 | 🟡 检测有，接入缺 | **P0 严重** |
| 5 | 扫仓 + 导入 + 初始记忆生成 | Onboarding PRD §3.4 | 🔴 代码存在但未调用 | **P0 致命** |
| 6 | Web 首次审阅向导 | Onboarding PRD §3.6 | 🔴 完全缺失 | **P0 严重** |
| 7 | Aha demo 对比 | Onboarding PRD §3.7 | 🔴 完全缺失 | **P0 严重** |
| 8 | 捕获引擎（显式信号） | MVP PRD §5.1 | 🟡 Claude Code only | **P0 中等** |
| 9 | 提炼/结晶引擎 | MVP PRD §5.2 | 🟡 引擎有但未接线 | **P0 致命** |
| 10 | 适配器写入 | MVP PRD §6 | 🟡 代码有但未自动触发 | **P0 严重** |
| 11 | Web 管理界面（查看/编辑/删除规则） | MVP PRD §7.3 | 🟡 列表有，详情缺 | **P0 中等** |
| 12 | 审核待生效规则 | MVP PRD §7.3 | 🔴 缺失 | **P0 严重** |
| 13 | 溯源（规则从哪来） | MVP PRD §7.3 | 🔴 UI 缺失 | **P0 中等** |
| 14 | Agent 接入状态 | MVP PRD §7.3 | 🟡 硬编码展示 | **P0 中等** |
| 15 | 全局/项目开关切换 | MVP PRD §7.3 | 🔴 硬编码，不可操作 | **P0 中等** |
| 16 | 隐私护栏（密钥不入库等） | MVP PRD §5.1 | ✅ 基本实现 | - |
| 17 | 零登录本地可用 | MVP PRD §4.1 | ✅ 完成 | - |
| 18 | 卸载可回滚 | MVP PRD §13.5 | 🟡 框架有，clean-blocks 未实现 | **P0 中等** |

### P1 需求（MVP 后优先）

| # | 需求 | 来源 | 状态 |
|---|------|------|------|
| 1 | 云同步 + OAuth 登录 | MVP PRD §4.1 | 未实现（预期 P1） |
| 2 | 高级翻译官质量 | MVP PRD §5.2 | 未实现（预期 P1） |
| 3 | 知识图谱视图 | MVP PRD §7.2 | 未实现（预期 P1） |
| 4 | 规则健康度/冲突提示 | MVP PRD §8 | 未实现 |
| 5 | 导出分享 | MVP PRD §8 | 未实现 |
| 6 | 菜单栏 badge + 系统通知 | MVP PRD §13.4 | 未实现（需桌面 App） |

---

## 7. 关键差距与优先修复建议

### 🔴 P0 致命（产品核心价值断裂）

#### 差距 1：Onboarding TUI 未接入

**问题**：`OnboardingModel` 代码完整（onboarding.go + tui.go），但 `shadow start` 命令没有调用它。

**修复方案**：
```go
// cmd/shadow/main.go startCmd 中，替换 TODO:
// 之前: // TODO: SHADOW-013 — onboarding TUI
// 之后:
model := daemon.NewOnboardingModel(shadow.Version)
p := tea.NewProgram(model)
if _, err := p.Run(); err != nil {
    return fmt.Errorf("onboarding: %w", err)
}
```

**预计工作量**：0.5 天

#### 差距 2：管线未打通（信号 → 规则 → 写入）

**问题**：daemon.go 创建了 captureEngine 和 distillEngine，但没有连接它们。信号入库后不会自动结晶，结晶后不会自动写入 agent 文件。

**修复方案**：在 daemon.go 中添加定时蒸馏循环 + 适配器写入触发：
1. 每 N 分钟（可配置）检查未结晶的 sources
2. 调用 distillEngine 处理
3. 新生成的 active 规则 → 适配器写入
4. WebSocket 广播事件给 web UI

**预计工作量**：2 天

#### 差距 3：扫仓/导入未接入 onboarding

**问题**：Scanner 和 Importer 的完整代码存在（scanner.go），但 onboarding.go 的 `scanProject()` 只做了简单的文件存在检查。

**修复方案**：在 onboarding 的 Step 4 中调用 `Scanner.Scan()` + `Importer.ImportFile()`，将结果写入数据库。

**预计工作量**：1 天

### 🟡 P0 严重（核心体验不完整）

#### 差距 4：Web Onboarding 向导缺失

**问题**：无 `/welcome` 路由，无审阅页面，无 Aha demo。

**修复方案**：
1. 添加 React 路由 `/welcome`
2. 审阅页：调用 `/api/rules?status=candidate`，按 scope 分组展示
3. Aha demo：静态对比展示（可从候选规则中选取 1-2 条）
4. 完成后跳转 Dashboard

**预计工作量**：3 天

#### 差距 5：规则详情/时间线/审核/冲突页面缺失

**问题**：Web 只有 4 个基础页面，缺少 PRD 定义的 4 个关键页面。

**修复方案**（优先级排序）：
1. 规则详情页（含时间线溯源）：2 天
2. 待生效审核队列：1.5 天
3. 冲突解决面板：1.5 天
4. Agent 接入状态页：1 天

**总预计工作量**：6 天

#### 差距 6：Settings 页面全硬编码

**问题**：Settings.tsx 所有值是静态字符串，开关不可操作。

**修复方案**：
1. 连接 `/api/config` API
2. 添加 toggle 真实交互
3. Adapter 状态动态获取

**预计工作量**：2 天

### 🟢 P1（体验增强）

- Dashboard 真实数据 + 图表
- 多 agent 日志解析器（Cursor、Copilot）
- `shadow review` 命令
- 规则健康度提醒
- 跨项目复用建议

---

## 8. Onboarding 优化方案

基于以上分析，以下是优化后的 **完整用户操作流程** 描述。每个步骤标注了当前状态和需要修复的内容。

### 优化后完整流程

```
Step 0: 安装
    $ brew install shadow          ← 需要发布到 Homebrew
    用户心理：好奇、略带怀疑

Step 1: shadow start — 欢迎
    ┌────────────────────────────────────────────┐
    │ 👻 Shadow  v0.1.0 · MVP                    │
    │ ─────────────────────────────────────      │
    │ 你纠正一次，所有本地 coding agent 都记住。  │
    │                                            │
    │ 即将完成 4 步（约 60 秒）：                 │
    │   1  注册后台服务 (daemon)                  │
    │   2  授权访问范围                           │
    │   3  检测并接入本地 agent                   │
    │   4  扫描项目，生成初始记忆                 │
    │                                            │
    │ 你的数据默认只存本地。全程可跳过、可回滚。  │
    │                                            │
    │ ▸ 按 Enter 开始        q 退出               │
    └────────────────────────────────────────────┘

    用户操作：按 Enter
    用户心理：清晰、安全、知道要做什么
    [需修复]：接入 OnboardingModel 到 startCmd

Step 2: 授权范围
    ┌─ Step 2/4 · 授权访问范围 ──────────────────┐
    │ Shadow 需要读写以下内容才能工作：           │
    │  [✓] 读取项目代码与会话日志                 │
    │  [✓] 写入各 agent 原生上下文文件            │
    │ 🔒 默认保护（已为你开启）：                 │
    │  • 绝不入库：密钥 / token / 凭证 / .env     │
    │  • 默认只存「提炼后的规则」                  │
    │  • 排除目录：node_modules, .git, dist, .env*│
    │ ▸ Enter 接受推荐设置     a 高级配置          │
    └────────────────────────────────────────────┘

    用户操作：按 Enter（默认安全，零决策负担）
    用户心理：放心，隐私被尊重
    [需修复]：新增授权步骤到 onboarding flow

Step 3: Agent 检测
    ┌─ Step 3/4 · 接入本地 agent ────────────────┐
    │ 已在你的机器上检测到 3 个 coding agent：     │
    │  [✓] Claude Code  → CLAUDE.md              │
    │  [✓] Cursor       → .cursorrules           │
    │  [ ] Codex        → AGENTS.md (未检测到)    │
    │ ▸ Enter 一键接入                            │
    └────────────────────────────────────────────┘

    用户操作：按 Enter（默认全选检测到的）
    用户心理：透明，知道每个 agent 会写到哪
    [需修复]：实际执行适配器写入，显示结果

Step 4: 扫仓 + 初始记忆
    ┌─ Step 4/4 · 生成初始记忆 ──────────────────┐
    │ ⠹ 正在扫描当前仓库…                        │
    │  • 包管理器：pnpm（lockfile 推断）          │
    │  • 测试框架：vitest                         │
    │  • 已有 CLAUDE.md（12 行约定）→ 可导入      │
    │ ▸ Enter 生成草稿并继续                      │
    └────────────────────────────────────────────┘

    用户操作：按 Enter
    用户心理：惊喜！"它居然已经知道我的项目了"
    [需修复]：调用 Scanner.Scan() + Importer + ToRules() + 写入 DB

Step 5: 完成 + 移交 Web
    ┌─ ✓ Shadow 已就绪 ──────────────────────────┐
    │ 👻 2 个 agent 已接入 · 生成 5 条初始记忆    │
    │ ▸ 按 Enter 打开控制台                      │
    │   s 跳过，留在终端                          │
    │ 常用命令：                                  │
    │   shadow status    查看状态                 │
    │   shadow open      打开控制台               │
    │   shadow review    审阅候选规则             │
    └────────────────────────────────────────────┘

    用户操作：按 Enter 打开浏览器
    用户心理：有成就感，迫不及待想看结果
    [需修复]：调用 exec.Command("open", "http://localhost:7878/welcome")

Step 6: Web 审阅初始记忆
    浏览器自动打开 localhost:7878/welcome

    ┌──────────────────────────────────────────────┐
    │ 👻 Shadow · 欢迎                跳过审阅 ›   │
    │ ●━━━━━━●━━━━━━○   第 1 步：审阅初始记忆      │
    │                                              │
    │ 我们从你的项目里提炼了 5 条记忆：             │
    │                                              │
    │ [✓] 本项目使用 pnpm    来源: lockfile  0.95  │
    │ [✓] 测试框架使用 vitest 来源: 扫仓    0.88   │
    │ [✓] 导入自 CLAUDE.md   来源: 导入    1.00    │
    │ [ ] 项目使用 TypeScript 来源: tsconfig 0.70  │
    │                                              │
    │        [ 启用所选并继续 → ]                   │
    └──────────────────────────────────────────────┘

    用户操作：勾选想要启用的规则，点击 "启用所选并继续"
    用户心理：掌控感，每条规则可溯源
    [需修复]：新建 /welcome 路由 + 审阅组件

Step 7: Aha Demo
    ┌──────────────────────────────────────────────┐
    │ ●━━━━━━●━━━━━━●   第 2 步：对比效果          │
    │                                              │
    │ 没有 Shadow:          有 Shadow:              │
    │ $ npm install ...     $ pnpm add ...          │
    │ ✗ 用了 npm            ✓ 自动用 pnpm          │
    │                                              │
    │ ✨ 同样的错误，这次它一次就对了。             │
    │ [ 完成，进入控制台 → ]                        │
    └──────────────────────────────────────────────┘

    用户操作：点击 "进入控制台"
    用户心理：*"真牛逼！"* ← 这就是 PMF 的信号
    [需修复]：新建 demo 组件

Step 8: 日常使用 · 无感捕获
    用户正常 coding，Shadow 后台自动工作。

    关键体验节点：
    - 用户纠正 agent → Shadow 捕获 → 自动结晶 → 写入
    - 切换到另一个 agent → 规则已生效 → 不再犯同样错误
    - Web 控制台可看到 "为你避免了 N 次重复错误"

    [需修复]：打通 capture → distill → adapter 自动管线
```

### 体验设计原则建议

1. **渐进式信息展示**：默认值让用户一键通过，高级选项可展开
2. **即时反馈**：每步完成后给出可量化成果（"已接入 3 个 agent"）
3. **信任建立**：隐私承诺显式可见、数据位置透明、操作可回滚
4. **Aha 时刻**：扫仓结果让用户感到惊喜，demo 对比让用户感到震撼
5. **零摩擦退出**：每一步都可跳过、可返回、可跳到终端

---

## 附录：修复优先级路线图

### Sprint 1（3 天）— 让产品「能用」

| 任务 | 天数 | 影响 |
|------|------|------|
| 接入 Onboarding TUI 到 startCmd | 0.5 天 | 用户第一次运行就有引导 |
| 修复 onboarding 扫仓（调用 Scanner + Importer） | 1 天 | 用户看到初始记忆 |
| 打通 distill 循环（daemon.go 添加定时蒸馏） | 1.5 天 | 信号自动变为规则 |

### Sprint 2（5 天）— 让产品「好用」

| 任务 | 天数 | 影响 |
|------|------|------|
| 适配器自动写入触发 | 1 天 | 规则生效到 agent 文件 |
| Web /welcome 审阅页 | 2 天 | 用户可审阅初始记忆 |
| Web 规则详情 + 时间线 | 2 天 | 用户可溯源规则来源 |

### Sprint 3（5 天）— 让产品「爱用」

| 任务 | 天数 | 影响 |
|------|------|------|
| Web Aha Demo 对比 | 1.5 天 | 用户 "真牛逼" 时刻 |
| Web 审核队列 + 冲突解决 | 2 天 | 规则管理闭环 |
| Settings 真实交互 | 1 天 | 配置可操作 |
| Dashboard 真实数据 | 0.5 天 | 数据可视化 |

### Sprint 4（3 天）— 让产品「可靠」

| 任务 | 天数 | 影响 |
|------|------|------|
| `shadow review` CLI 命令 | 1 天 | 纯终端闭环 |
| 卸载回滚 managed blocks | 1 天 | 安全退出 |
| 集成测试 + E2E 测试 | 1 天 | 质量保障 |

---

> **总结**：Shadow 的架构基础和后端实现质量很高，数据模型设计合理，隐私保护到位。核心问题是 **用户体验链路的最后 10%**——onboarding 未接入、管线未打通、web 界面是骨架。这些是「看起来能跑」和「用起来真牛逼」之间的差距。按照上述 Sprint 路线图，约 16 个工作日可以将 MVP 推到 PMF 可验证状态。
