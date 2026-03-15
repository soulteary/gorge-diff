# gorge-diff 技术报告

## 1. 概述

gorge-diff 是 Gorge 平台中的文本差异计算微服务，为 Phorge（Phabricator 社区维护分支）提供两种文本差异计算能力：Unified Diff（行级差异）和 Prose Diff（多级细粒度文本差异）。

该服务的核心目标是将 Phorge PHP 端的两个 diff 引擎——`PhabricatorDifferenceEngine`（通过系统 `diff` 命令生成 unified diff）和 `PhutilProseDifferenceEngine`（纯 PHP 实现的多级文本 diff）——抽取为独立的 Go HTTP 微服务。Go 的原生字符串处理性能远优于 PHP，且不再依赖系统 `diff` 命令，消除了对外部进程的依赖。

## 2. 设计动机

### 2.1 原有方案的问题

Phorge 的 diff 功能嵌入在 PHP 应用中，存在以下问题：

1. **外部进程依赖**：`PhabricatorDifferenceEngine` 通过 `diff -U65535` 调用系统 `diff` 命令，依赖宿主环境安装了 GNU diff。每次 diff 计算都需要 `proc_open` 创建子进程、通过管道传递文本、等待进程退出并解析输出，开销显著。
2. **PHP 性能瓶颈**：`PhutilProseDifferenceEngine` 是纯 PHP 实现的多级递归 diff 算法，涉及大量字符串拆分、动态规划矩阵构建和递归调用。PHP 的解释执行模型在 CPU 密集型计算场景下性能不佳。
3. **阻塞请求**：diff 计算在 PHP 请求生命周期内同步执行，大文本的 diff 会阻塞 PHP-FPM 进程，影响其他请求的处理。
4. **不可独立扩展**：diff 计算与 Phorge PHP 应用绑定在同一部署单元中，无法根据 diff 计算负载独立扩容。

### 2.2 gorge-diff 的解决思路

将 diff 计算抽取为独立的 Go HTTP 微服务：

- **消除外部依赖**：unified diff 通过 Go 原生 LCS 算法实现，不再依赖系统 `diff` 命令。
- **语言级加速**：Go 的编译执行、值类型字符串和高效的内存分配器，在字符串密集计算场景下性能远超 PHP。
- **异步解耦**：Phorge PHP 端通过 HTTP 调用 gorge-diff，diff 计算不再阻塞 PHP-FPM 进程。
- **独立扩展**：作为独立容器运行，可根据 diff 计算负载独立扩缩容。
- **行为兼容**：两种 diff 算法的输出格式与 PHP 原始实现完全一致，确保下游系统无缝切换。

## 3. 系统架构

### 3.1 在 Gorge 平台中的位置

```
┌──────────────────────────────────────────────────┐
│                   Gorge 平台                      │
│                                                   │
│  ┌──────────┐  ┌──────────┐  ┌───────────────┐   │
│  │  Phorge  │  │  gorge-  │  │ 其他 Go 服务   │   │
│  │  (PHP)   │  │  worker  │  │               │   │
│  └────┬─────┘  └────┬─────┘  └───────┬───────┘   │
│       │              │                │           │
│       └──────────────┼────────────────┘           │
│                      │                            │
│                      ▼                            │
│          ┌───────────────────────┐                │
│          │    gorge-diff        │                │
│          │    :8130             │                │
│          │                     │                │
│          │    Token Auth       │                │
│          │    Unified Diff     │                │
│          │    Prose Diff       │                │
│          └───────────────────────┘                │
│                                                   │
│          纯计算服务，无外部存储依赖                  │
└──────────────────────────────────────────────────┘
```

### 3.2 模块划分

项目采用 Go 标准布局，分为三个内部模块：

| 模块 | 路径 | 职责 |
|---|---|---|
| config | `internal/config/` | 环境变量配置加载 |
| engine | `internal/engine/` | Unified Diff 和 Prose Diff 核心算法 |
| httpapi | `internal/httpapi/` | HTTP 路由注册、认证中间件与请求处理 |

入口程序 `cmd/server/main.go` 负责串联三个模块：加载配置 -> 初始化 Echo 和中间件 -> 注册路由 -> 启动 HTTP 服务。

### 3.3 请求处理流水线

一个 diff API 请求经过的完整处理链路：

```
客户端请求 POST /api/diff/generate
       │
       ▼
┌─ Echo 框架层 ─────────────────────────────────┐
│  RequestLogger  记录请求方法、URI、状态码         │
│       │                                        │
│       ▼                                        │
│  Recover        捕获 panic，防止进程崩溃          │
│       │                                        │
│       ▼                                        │
│  BodyLimit      拒绝超过 MaxBodySize 的请求体     │
└───────┼────────────────────────────────────────┘
        │
        ▼
┌─ 路由组 /api/diff ────────────────────────────┐
│  tokenAuth       校验 X-Service-Token           │
│       │                                        │
│       ▼                                        │
│  Handler         绑定 JSON → 调用 Engine         │
└───────┼────────────────────────────────────────┘
        │
        ▼
┌─ Engine 层 ──────────────────────────────────┐
│  GenerateUnifiedDiff  或  GenerateProseDiff    │
│  纯内存计算，无 I/O                             │
└───────┼────────────────────────────────────────┘
        │
        ▼
  APIResponse{Data: DiffResult/ProseResult}
  返回客户端
```

## 4. 核心实现分析

### 4.1 Unified Diff 引擎

Unified Diff 引擎位于 `internal/engine/unified.go`，实现了与 `PhabricatorDifferenceEngine::generateRawDiffFromFileContent` 兼容的行级差异计算。

#### 4.1.1 处理流程

`GenerateUnifiedDiff` 方法的执行步骤：

1. **文件名处理**：如果请求未提供 `oldName`/`newName`，使用默认值 `/dev/universe`。
2. **文本规范化**（可选）：当 `Normalize` 为 true 时，去除所有空格和 Tab，使 diff 忽略空白差异。
3. **行拆分**：按 `\n` 拆分为行数组，去除末尾空行。
4. **快速路径**：如果两个行数组完全相同，直接生成全上下文的 identical diff 并返回 `Equal: true`。
5. **LCS 计算**：调用 `lcs()` 计算编辑操作序列。
6. **格式化输出**：调用 `formatUnified()` 生成 unified diff 文本。

#### 4.1.2 LCS 算法

`lcs()` 函数使用经典的最长公共子序列（Longest Common Subsequence）动态规划算法：

```go
dp := make([][]int, n+1)
for i := 1; i <= n; i++ {
    for j := 1; j <= m; j++ {
        if a[i-1] == b[j-1] {
            dp[i][j] = dp[i-1][j-1] + 1
        } else if dp[i-1][j] >= dp[i][j-1] {
            dp[i][j] = dp[i-1][j]
        } else {
            dp[i][j] = dp[i][j-1]
        }
    }
}
```

算法分两个阶段：

**填表阶段**：构造 `(n+1) × (m+1)` 的二维 DP 表，其中 `n = len(oldLines)`，`m = len(newLines)`。对于每个位置 `(i, j)`：
- 如果 `a[i-1] == b[j-1]`，则 `dp[i][j] = dp[i-1][j-1] + 1`（两行匹配，LCS 长度 +1）
- 否则取 `max(dp[i-1][j], dp[i][j-1])`（跳过旧行或新行中的某一行）

**回溯阶段**：从 `dp[n][m]` 开始反向追踪最优路径，生成编辑操作序列：

```go
for i > 0 || j > 0 {
    if i > 0 && j > 0 && a[i-1] == b[j-1] {
        ops = append(ops, opEqual)  // '=' 两行相同
        i--; j--
    } else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
        ops = append(ops, opInsert) // '+' 新文件中新增行
        j--
    } else {
        ops = append(ops, opDelete) // '-' 旧文件中删除行
        i--
    }
}
```

回溯产生的操作序列是逆序的，最后通过原地反转得到正序的编辑脚本。

时间复杂度 O(n × m)，空间复杂度 O(n × m)。在代码审查场景中，待比较的文件通常不超过数千行，这个复杂度完全可接受。

#### 4.1.3 边界情况处理

算法对三种边界情况做了快速处理：

- **两者都为空**：`splitLines("")` 返回 `nil`，`linesEqual(nil, nil)` 为 true，走快速路径。
- **旧文件为空**：`lcs(nil, newLines)` 直接生成全 `opInsert` 序列，无需 DP 计算。
- **新文件为空**：`lcs(oldLines, nil)` 直接生成全 `opDelete` 序列，无需 DP 计算。

#### 4.1.4 Unified 格式化

`formatUnified()` 将编辑操作序列转换为标准 Unified Diff 格式：

```
--- oldName 9999-99-99
+++ newName 9999-99-99
@@ -1,oldCount +1,newCount @@
 unchanged line
-deleted line
+inserted line
```

格式化的几个要点：

- **固定时间戳**：使用 `9999-99-99` 作为时间戳，与 Phorge PHP 端行为一致。真实时间戳在代码审查场景中无意义，且会导致相同输入产生不同输出。
- **全上下文模式**：只生成一个 hunk，起始行号始终为 1，相当于 `diff -U65535`。这是 Phorge 所需的格式——下游代码会对 diff 做进一步解析和渲染，需要完整的上下文信息。
- **行前缀**：空格表示不变行，`-` 表示删除行，`+` 表示插入行。

### 4.2 Prose Diff 引擎

Prose Diff 引擎位于 `internal/engine/prose.go`，实现了与 `PhutilProseDifferenceEngine::getDiff` 兼容的多级细粒度文本差异计算。这是整个项目中最复杂的算法模块。

#### 4.2.1 四级递归架构

Prose Diff 的核心设计是四级递归——从粗粒度逐步细化到精确的差异定位：

```
Level 0: 段落级（按 \n+ 分割）── hashDiff 贪心对齐
    │
    ▼ 对变更块递归
Level 1: 句子级（按标点 .!?;, 分割）── editDistanceDiff
    │
    ▼ 对变更块递归
Level 2: 词级（按空白符分割）── editDistanceDiff + smooth
    │
    ▼ 对变更块递归
Level 3: 字符级（按 UTF-8 rune 分割）── editDistanceDiff + smooth
```

每个级别的工作流程相同：

1. 使用该级别的分割策略将文本拆分为片段
2. 使用该级别对应的 diff 算法比较片段
3. 将结果中连续的 `-`/`+` 聚合为"变更块"
4. 对每个变更块，递归调用下一级别进行更精细的 diff
5. Level 3 是递归终止条件，直接返回结果

这种设计的优势在于：段落级比较可以快速跳过未变更的大段落，只在真正发生变更的区域才进入更精细的比较层级，兼顾了性能和精度。

#### 4.2.2 文本分割策略

`splitCorpus()` 函数根据级别选择不同的分割正则：

| 级别 | 正则表达式 | 分割单位 | 说明 |
|---|---|---|---|
| Level 0 | `(\n+)` | 段落 | 按一个或多个换行符分割 |
| Level 1 | `([\n,!;?.]+)` | 句子 | 按标点符号和换行符分割 |
| Level 2 | `(\s+)` | 词 | 按空白字符分割 |
| Level 3 | — | 字符 | 逐 UTF-8 rune 拆分 |

正则使用捕获组 `(...)` 以保留分隔符，通过 `stitchPieces()` 将内容片段和分隔符重新拼接。这确保了分割后的片段拼接起来能完全还原原始文本——diff 结果中的文本不会丢失任何字符。

#### 4.2.3 stitchPieces 与 trimApart

`stitchPieces()` 负责将 `Split` 产生的内容片段与 `FindAllString` 产生的分隔符重新组合：

```go
func stitchPieces(parts []string, delims []string, level int) []string {
    var results []string
    for i, part := range parts {
        piece := part
        if i < len(delims) {
            piece += delims[i]
        }
        if level < 2 {
            trimmed := trimApart(piece)
            results = append(results, trimmed...)
        } else {
            results = append(results, piece)
        }
    }
    // ...
}
```

对于 Level 0 和 Level 1，还会通过 `trimApart()` 将每个片段拆分为 `[前导空白, 正文, 尾随空白]` 三部分。这样空白字符作为独立元素参与 diff，可以被精确地标记为不变/变更，避免在变更区域的边界出现多余的空白。

#### 4.2.4 hashDiff — 段落级贪心对齐

`hashDiff()` 用于 Level 0，模仿 `PhutilProseDifferenceEngine::newHashDiff`：

```go
func hashDiff(uParts, vParts []string) []ProsePart {
    vMap := make(map[string][]int)
    for i, p := range vParts {
        vMap[p] = append(vMap[p], i)
    }
    // 贪心前向匹配
    vNext := 0
    for i, up := range uParts {
        indices := vMap[up]
        for _, vi := range indices {
            if vi >= vNext && !vUsed[vi] {
                matches[i] = vi
                vUsed[vi] = true
                vNext = vi + 1
                break
            }
        }
    }
    // ...
}
```

算法步骤：

1. **建立索引**：为新文本的所有段落建立 `map[string][]int`（内容 → 位置列表）。
2. **贪心匹配**：按顺序扫描旧文本段落，对每个段落在新文本中查找第一个可用的相同内容段落，匹配后标记已使用并更新 `vNext` 游标，保证匹配顺序递增。
3. **生成 diff**：遍历匹配结果，未匹配的旧段落标记为 `-`，未匹配的新段落标记为 `+`，匹配的标记为 `=`。

选择 hashDiff 而非 editDistanceDiff 用于段落级的原因：段落数量通常较少但每个段落可能很长，内容相同的段落直接通过字符串哈希匹配，复杂度接近 O(n)，远优于编辑距离的 O(n × m)。

#### 4.2.5 editDistanceDiff — 编辑距离对齐

`editDistanceDiff()` 用于 Level 1/2/3，实现标准的 Levenshtein 编辑距离算法：

```go
for i := 1; i <= n; i++ {
    for j := 1; j <= m; j++ {
        if uParts[i-1] == vParts[j-1] {
            dp[i][j] = dp[i-1][j-1]         // 相同，代价 0
        } else {
            sub := dp[i-1][j-1] + 1          // 替换
            del := dp[i-1][j] + 1            // 删除
            ins := dp[i][j-1] + 1            // 插入
            dp[i][j] = min3(sub, del, ins)
        }
    }
}
```

回溯阶段生成四种操作码：

- `s` (same)：片段完全相同，输出为 `=`
- `x` (substitute)：替换，输出为 `-` 旧片段 + `+` 新片段
- `d` (delete)：删除，输出为 `-`
- `i` (insert)：插入，输出为 `+`

区分 `s` 和 `x` 的意义在于后续的 smooth 处理——替换操作两端如果相邻的片段也是变更操作，可以将中间孤立的相同片段合并为替换，减少输出碎片。

#### 4.2.6 安全阈值

```go
const maxEditDistance = 128

tooLarge := n > maxEditDistance || m > maxEditDistance
if tooLarge {
    // 直接输出全删+全插
}
```

当输入片段数超过 128 时，编辑距离矩阵将达到 128 × 128 = 16,384 个单元，继续增长会消耗过多内存和 CPU 时间。此时算法退化为"全部删除旧内容 + 全部插入新内容"，并将 `tooLarge` 标志返回给调用方。

调用方收到 `tooLarge` 标志后会跳过对变更块的递归细化——既然当前级别的 diff 就已经无法精确计算，更细粒度的递归也无意义。

#### 4.2.7 smooth — 平滑处理

`smooth()` 函数用于 Level 2（词级）和 Level 3（字符级），消除输出中的孤立相同片段：

```go
func smooth(ops []byte) []byte {
    result := make([]byte, len(ops))
    copy(result, ops)
    for i := range result {
        if result[i] != 's' {
            continue
        }
        prevChange := i > 0 && result[i-1] != 's'
        nextChange := i < len(result)-1 && result[i+1] != 's'
        if prevChange && nextChange {
            result[i] = 'x'  // 转换为替换
        }
    }
    return result
}
```

转换条件：一个 `s`（相同）操作，如果前一个操作不是 `s` 且后一个操作也不是 `s`，则将其转换为 `x`（替换）。

例如操作序列 `d s i` 会被平滑为 `d x i`。效果是：在"The **quick** brown **fox**"变为"The **slow** brown **cat**"的 diff 中，如果"brown"前后的单词都发生了变更，不做平滑会产生 5 个片段（`-quick`, `=brown`, `+slow`, `-fox`, `+cat`），平滑后可能产生更连贯的输出。

Level 1（句子级）不做平滑，因为句子是较大的语义单位，保留孤立的相同句子对可读性更好。

#### 4.2.8 reorderParts — 重排合并

`reorderParts()` 是每个递归级别返回前的后处理步骤，执行两个操作：

**排序**：将连续的 `-` 和 `+` 操作分组，确保所有删除操作出现在插入操作之前。这是因为 diff 结果中 `+old -new` 交错出现时可读性差，统一为 `-old +new` 顺序更自然。

```go
for _, p := range parts {
    switch p.Type {
    case "-":
        oRun = append(oRun, p)
    case "+":
        nRun = append(nRun, p)
    default:
        flush()  // 遇到 "=" 时，将累积的 oRun 和 nRun 输出
        result = append(result, p)
    }
}
```

**合并**：将相邻的同类型操作合并为一个——多个连续的 `-` 合并为一个 `-`，多个连续的 `+` 合并为一个 `+`，多个连续的 `=` 合并为一个 `=`。

#### 4.2.9 combineRuns — 布局字符提取

`combineRuns()` 在合并删除/插入序列时，智能提取公共的布局字符前缀和后缀：

```go
var layoutChars = map[byte]bool{
    ' ': true, '\n': true, '.': true, '!': true, ',': true,
    '?': true, ']': true, '[': true, '(': true, ')': true,
    '<': true, '>': true,
}
```

算法同时从删除文本和插入文本的头部和尾部扫描，如果两者的前缀/后缀是相同的布局字符（空格、标点等），就将这部分提取为 `=`（不变）部分。

例如 `-" hello"` 和 `+" world"` 会被拆分为 `=" "` + `-"hello"` + `+"world"`。这样在 UI 渲染时，空格不会被标记为变更，视觉效果更自然。

仅提取**布局字符**而非所有公共前缀的原因：普通字符的公共前缀可能产生误导。例如 `-"abcX"` 和 `+"abcY"`，如果提取 `="abc"` 则暗示 `abc` 是有意保留的，但实际上整个词都被替换了。而空格和标点是格式字符，将其标记为不变是合理的。

### 4.3 数据模型

数据模型位于 `internal/engine/model.go`，定义了两组请求/响应结构。

#### 4.3.1 Unified Diff

```go
type DiffRequest struct {
    Old       string `json:"old"`
    New       string `json:"new"`
    OldName   string `json:"oldName,omitempty"`
    NewName   string `json:"newName,omitempty"`
    Normalize bool   `json:"normalize,omitempty"`
}

type DiffResult struct {
    Diff  string `json:"diff"`
    Equal bool   `json:"equal"`
}
```

- `Old`/`New`：待比较的完整文本内容。
- `OldName`/`NewName`：文件名，用于 unified diff 头部。可选，默认为 `/dev/universe`。
- `Normalize`：是否在比较前去除空格和 Tab，用于忽略空白差异的场景。
- `Diff`：生成的 unified diff 文本。
- `Equal`：布尔标志，快速判断两个文本是否相同，下游可以据此跳过 diff 解析。

#### 4.3.2 Prose Diff

```go
type ProseRequest struct {
    Old string `json:"old"`
    New string `json:"new"`
}

type ProsePart struct {
    Type string `json:"type"`  // "=", "-", "+"
    Text string `json:"text"`
}

type ProseResult struct {
    Parts []ProsePart `json:"parts"`
}
```

- `Parts`：有序的差异片段列表，每个片段包含类型（`=` 不变 / `-` 删除 / `+` 插入）和对应的文本内容。
- 所有 `Parts` 的文本拼接起来，`=` 和 `-` 的文本还原旧文本，`=` 和 `+` 的文本还原新文本。

### 4.4 HTTP 层

#### 4.4.1 路由设计

```go
func RegisterRoutes(e *echo.Echo, deps *Deps) {
    e.GET("/", healthPing())
    e.GET("/healthz", healthPing())

    g := e.Group("/api/diff")
    g.Use(tokenAuth(deps))

    g.POST("/generate", generateDiff())
    g.POST("/prose", proseDiff())
}
```

路由设计的几个要点：

- **健康检查独立**：`/` 和 `/healthz` 不经过认证中间件，确保 Docker HEALTHCHECK 和负载均衡器的探测不受影响。
- **路由组中间件**：`tokenAuth` 作为 `/api/diff` 路由组的中间件，仅对受保护端点生效。
- **POST 方法**：diff 请求携带文本内容，使用 POST 方法传递 JSON body，避免 URL 长度限制和编码问题。

#### 4.4.2 认证中间件

```go
func tokenAuth(deps *Deps) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            if deps.Token == "" {
                return next(c)
            }
            token := c.Request().Header.Get("X-Service-Token")
            if token == "" {
                token = c.QueryParam("token")
            }
            if token == "" || token != deps.Token {
                return c.JSON(http.StatusUnauthorized, ...)
            }
            return next(c)
        }
    }
}
```

设计要点：

- **可选认证**：当 `SERVICE_TOKEN` 环境变量为空时，中间件直接放行所有请求，适合开发和测试环境。
- **双通道获取**：支持 `X-Service-Token` 请求头和 `?token=` 查询参数两种方式。请求头适合服务间调用，查询参数适合调试。
- **优先级**：先检查请求头，未找到时再检查查询参数。

#### 4.4.3 统一响应格式

```go
type apiResponse struct {
    Data  any       `json:"data,omitempty"`
    Error *apiError `json:"error,omitempty"`
}

type apiError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

所有 API 响应遵循统一的信封格式：

- **成功**：`{"data": {...}}`，`data` 字段包含 `DiffResult` 或 `ProseResult`。
- **错误**：`{"error": {"code": "ERR_XXX", "message": "..."}}`，结构化的错误码和描述信息。

错误码覆盖两种场景：

| 错误码 | HTTP 状态码 | 含义 |
|---|---|---|
| `ERR_UNAUTHORIZED` | 401 | Service Token 缺失或不匹配 |
| `ERR_BAD_REQUEST` | 400 | 请求体 JSON 解析失败 |

#### 4.4.4 Handler 实现

两个 Handler 遵循相同的模式：绑定 JSON 请求体，调用 Engine 层函数，返回结果。

```go
func generateDiff() echo.HandlerFunc {
    return func(c echo.Context) error {
        var req engine.DiffRequest
        if err := c.Bind(&req); err != nil {
            return respondErr(c, http.StatusBadRequest, "ERR_BAD_REQUEST", err.Error())
        }
        result := engine.GenerateUnifiedDiff(&req)
        return respondOK(c, result)
    }
}
```

Engine 层函数是纯计算函数，不会返回错误（输入为空时返回空结果），Handler 层唯一可能的错误来源是 JSON 绑定失败。

### 4.5 中间件栈

入口程序 `cmd/server/main.go` 配置了三层全局中间件：

```go
e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
    LogURI:    true,
    LogStatus: true,
    LogMethod: true,
    LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
        c.Logger().Infof("%s %s %d", v.Method, v.URI, v.Status)
        return nil
    },
}))
e.Use(middleware.Recover())
e.Use(middleware.BodyLimit(cfg.MaxBodySize))
```

| 中间件 | 作用 |
|---|---|
| RequestLogger | 记录每个请求的方法、URI 和状态码，用于运维监控和问题排查 |
| Recover | 捕获 Handler 中的 panic，返回 500 而非导致进程崩溃 |
| BodyLimit | 限制请求体大小（默认 10MB），防止大文本消耗过多内存和 CPU |

BodyLimit 中间件在 diff 服务中尤为重要——diff 算法的时间和空间复杂度与输入大小成二次关系（LCS 为 O(n × m)），不限制输入大小可能导致单个请求消耗大量资源。

### 4.6 配置模块

配置模块位于 `internal/config/config.go`，所有配置通过环境变量加载。

```go
type Config struct {
    ListenAddr   string
    ServiceToken string
    MaxBodySize  string
}

func LoadFromEnv() *Config {
    return &Config{
        ListenAddr:   envStr("LISTEN_ADDR", ":8130"),
        ServiceToken: envStr("SERVICE_TOKEN", ""),
        MaxBodySize:  envStr("MAX_BODY_SIZE", "10M"),
    }
}
```

配置项简洁明了：

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `LISTEN_ADDR` | `:8130` | 监听地址 |
| `SERVICE_TOKEN` | (空) | 服务间认证令牌，为空则跳过校验 |
| `MAX_BODY_SIZE` | `10M` | 请求体大小限制 |

仅支持环境变量而不支持配置文件的设计选择：服务只有 3 个配置项，环境变量完全够用，且与 Docker 和 Kubernetes 的配置注入方式天然匹配。

## 5. 与 PHP 原始实现的对应关系

| Go 实现 | PHP 原始实现 | 差异说明 |
|---|---|---|
| `GenerateUnifiedDiff()` | `PhabricatorDifferenceEngine::generateRawDiffFromFileContent()` | PHP 版调用系统 `diff -U65535`，Go 版纯代码实现 LCS，消除外部进程依赖 |
| `GenerateProseDiff()` | `PhutilProseDifferenceEngine::getDiff()` | 完全兼容的四级递归 diff，算法逻辑一致 |
| `hashDiff()` | `PhutilProseDifferenceEngine::newHashDiff()` | 段落级贪心对齐，行为一致 |
| `editDistanceDiff()` | `PhutilProseDifferenceEngine::newEditDistanceMatrixDiff()` | 编辑距离 DP，增加了 `maxEditDistance=128` 安全阈值 |
| `reorderParts()` | `PhutilProseDiff::reorderParts()` | 后处理重排合并，行为一致 |
| `combineRuns()` | `PhutilProseDiff::reorderParts()` 内部逻辑 | 布局字符提取，行为一致 |
| `smooth()` | PHP 内联逻辑 | 词/字符级孤立相同片段平滑 |
| `splitCorpus()` | `PhutilProseDifferenceEngine::splitCorpus()` | 四级分割策略，正则一致 |
| `trimApart()` | `PhutilProseDifferenceEngine::trimApart()` | 空白字符拆分，行为一致 |

## 6. 部署方案

### 6.1 Docker 镜像

采用多阶段构建：

- **构建阶段**：基于 `golang:1.26-alpine3.22`，使用 `CGO_ENABLED=0` 静态编译，`-ldflags="-s -w"` 去除调试信息和符号表以缩小二进制体积。
- **运行阶段**：基于 `alpine:3.20`，仅包含编译后的二进制和 CA 证书。

内置 Docker `HEALTHCHECK`，每 10 秒通过 `wget` 检查 `/healthz` 端点，启动等待 5 秒，超时 3 秒，最多重试 3 次。

### 6.2 资源控制

多层资源保护机制防止滥用：

| 层级 | 机制 | 默认值 | 作用 |
|---|---|---|---|
| Echo 框架 | `BodyLimit` 中间件 | 10 MB | 拒绝超大请求体 |
| 认证层 | Service Token | (可选) | 阻止未授权访问 |
| 算法层 | `maxEditDistance` | 128 | 编辑距离矩阵不超过 128×128，防止大输入 OOM |

## 7. 依赖分析

| 依赖 | 版本 | 用途 |
|---|---|---|
| `labstack/echo/v4` | v4.15.1 | HTTP 框架，提供路由、中间件和上下文管理 |
| `golang.org/x/crypto` | v0.49.0 | Echo 的加密基础（间接） |
| `golang.org/x/net` | v0.52.0 | Echo 的网络基础（间接） |
| `golang.org/x/time` | v0.15.0 | Echo 的时间工具（间接） |

直接依赖仅 Echo 一个。两个 diff 引擎完全基于 Go 标准库实现——字符串操作使用 `strings` 包，Unicode 处理使用 `unicode/utf8` 包，正则表达式使用 `regexp` 包。不引入第三方 diff 库，保持了代码的可控性和对 Phorge 行为的精确兼容。

## 8. 测试覆盖

项目包含四组测试文件，覆盖所有核心模块：

| 测试文件 | 覆盖范围 |
|---|---|
| `config_test.go` | 默认配置值验证、自定义环境变量覆盖 |
| `unified_test.go` | 相同输入、变更/添加/删除、空输入、默认文件名、Normalize 空格/Tab、行拆分、LCS 全插入/全删除/混合/完全替换/双空、行比较、identical diff 单行/空、formatUnified 纯插入/纯删除、多行变更、纯新/纯旧内容 |
| `prose_test.go` | 相同/简单变更/添加/空输入、旧空/新空、reorderParts 排序/合并/空/纯等号/混合序列、trimApart 各种空白组合、splitCorpus 段落/句子/词/字符/默认级别/空输入、splitChars Unicode/空、editDistanceDiff 正常/相同/纯插入/纯删除/超大阈值/有平滑/无平滑、isVMatched、hashDiff 重排/未匹配新段落/全新、combineRuns 前缀/后缀/无公共布局字符/纯删除/纯插入、mergeRunText 正常/空、min3 各种组合、smooth 孤立/连续/全同/边缘位置、buildProseDiff 双空/Level3 |
| `handlers_test.go` | 健康检查（/ 和 /healthz）、Token 认证（无 Token 401/正确 Token/Query 参数/禁用认证/错误 Token/错误 Query/空 Query/Prose 带 Token）、generate 端点（正常请求/错误 JSON/相同输入/响应结构/错误响应结构）、prose 端点（正常请求/错误 JSON/相同输入/响应结构） |

测试设计的几个特点：

- **表驱动测试**：`unified_test.go` 和 `prose_test.go` 中的辅助函数测试（`splitLines`、`linesEqual`、`normalizeText`、`trimApart`、`min3`、`smooth`）均采用 table-driven 风格，覆盖多种输入组合。
- **算法层独立测试**：LCS、编辑距离、hashDiff、reorderParts、combineRuns 等算法函数都有独立的单元测试，不依赖 HTTP 层。
- **端到端验证**：`handlers_test.go` 使用 `httptest` 构建完整的 Echo 服务实例，从 HTTP 请求到 JSON 响应进行端到端验证。
- **边界覆盖**：空输入、纯插入、纯删除、超大输入阈值、Unicode 字符等边界场景均有覆盖。

## 9. 总结

gorge-diff 是一个职责单一的文本差异计算微服务，核心价值在于：

1. **消除外部依赖**：unified diff 从系统 `diff` 命令调用变为纯 Go LCS 实现，prose diff 从 PHP 解释执行变为 Go 编译执行，消除了对宿主环境和 PHP 运行时的依赖。
2. **多级递归 diff**：Prose Diff 通过段落→句子→词→字符四级递归架构，在保持性能的同时获得人类友好的细粒度差异输出。粗粒度级别快速跳过未变更区域，精细级别只在变更区域内工作。
3. **安全边界**：编辑距离矩阵限制 128×128、请求体大小限制 10MB、可选的 Token 认证，多层保护防止资源耗尽和未授权访问。
4. **行为兼容**：所有算法对标 Phorge PHP 实现——分割正则、操作码语义、重排逻辑、布局字符提取、平滑规则均保持一致，确保替换后输出一致。
5. **极简实现**：仅依赖 Echo 框架，两个 diff 引擎完全基于标准库实现，代码量约 500 行（不含测试），易于理解和维护。
