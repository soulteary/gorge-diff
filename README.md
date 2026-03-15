# github.com/soulteary/gorge-diff

Go 微服务，替代 Phorge PHP 中的 `PhabricatorDifferenceEngine`（系统 `diff` 调用）和 `PhutilProseDifferenceEngine`（文本级 prose diff）。

## 功能

| 端点 | 方法 | 说明 |
|------|------|------|
| `/healthz` | GET | 健康检查 |
| `/api/diff/generate` | POST | 生成 unified diff（全上下文），兼容 `diff -U65535` 输出 |
| `/api/diff/prose` | POST | 生成 prose diff（段落→句子→词→字符 多级细粒度） |

## 配置

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `LISTEN_ADDR` | `:8130` | 监听地址 |
| `SERVICE_TOKEN` | _(空)_ | 服务间认证令牌，为空则跳过校验 |
| `MAX_BODY_SIZE` | `10M` | 请求体大小限制 |

## API 示例

### 生成 Unified Diff

```bash
curl -X POST http://localhost:8130/api/diff/generate \
  -H 'Content-Type: application/json' \
  -d '{"old":"hello\nworld\n","new":"hello\ngopher\n","oldName":"a.txt","newName":"b.txt"}'
```

响应：

```json
{
  "data": {
    "diff": "--- a.txt 9999-99-99\n+++ b.txt 9999-99-99\n@@ -1,2 +1,2 @@\n hello\n-world\n+gopher\n",
    "equal": false
  }
}
```

### 生成 Prose Diff

```bash
curl -X POST http://localhost:8130/api/diff/prose \
  -H 'Content-Type: application/json' \
  -d '{"old":"The quick brown fox","new":"The slow brown cat"}'
```

响应：

```json
{
  "data": {
    "parts": [
      {"type": "=", "text": "The "},
      {"type": "-", "text": "quick"},
      {"type": "+", "text": "slow"},
      {"type": "=", "text": " brown "},
      {"type": "-", "text": "fox"},
      {"type": "+", "text": "cat"}
    ]
  }
}
```

## 开发

```bash
go test ./...
go run ./cmd/server
```

## Docker

```bash
docker build -t github.com/soulteary/gorge-diff .
docker run -p 8130:8130 github.com/soulteary/gorge-diff
```
