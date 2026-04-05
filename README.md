# Broadcast Local Backend (Go)

本项目当前采用“本地转录 + Dify 编排 + 本地剪辑”架构：

1. 本地 CLI 对音频/视频做 ASR（支持长音频分片）。
2. 输出 `transcript.json` 给 Dify 工作流做多 Agent 爆点提取。
3. Dify 回调本地 `/v1/video/clip` 执行 FFmpeg 剪辑。

## 工作流总览

![AI 播客生产 Agent 工作流](public/AI%20播客生产%20Agent-whole-workflow.png)

## 当前能力

1. `cmd/asr-cli`：单文件转录，支持长音频分片（`chunk + overlap + retry`）。
2. `cmd/server`：轻量 API 服务，提供健康检查和视频剪辑接口。
3. `/v1/video/clip`：按 `start/end` 批量裁剪视频片段并返回输出路径。

## 目录

```text
cmd/server/main.go
cmd/asr-cli/main.go
internal/api/router.go
internal/asr/client.go
internal/clip/service.go
internal/media/service.go
internal/config/config.go
internal/model/
```

## 环境变量

复制 `.env.example` 后至少配置：

```env
SERVER_PORT=8088
APP_ENV=dev
LOG_LEVEL=info
WORK_DIR=./data
FFMPEG_BIN=ffmpeg
FFPROBE_BIN=ffprobe

ASR_BASE_URL=https://api.oepnai.com
ASR_API_KEY=your_key
ASR_MODEL=whisper-1
ASR_CHUNK_SECONDS=600
ASR_CHUNK_OVERLAP_SECONDS=2
ASR_RETRY_COUNT=3
```

## 本地运行

### 1) 启动服务端

```bash
cd /Users/firefly/Desktop/agent/broadcast
cp .env.example .env
go run ./cmd/server
```

健康检查：

```bash
curl http://127.0.0.1:8088/healthz
```

### 2) 执行转录（CLI）

```bash
go run ./cmd/asr-cli --file ./data/input/video/test.mp4 --type video
```

参数：

1. `--file`：必填，输入文件路径。
2. `--type`：`auto|audio|video`，默认 `auto`。
3. `--model`：可选，覆盖 `ASR_MODEL`。
4. `--output`：可选，指定输出 JSON 路径。

默认输出：

`./data/output/transcripts/<文件名>.transcript.json`

## 视频剪辑接口

`POST /v1/video/clip`

请求示例：

```json
{
  "video_path": "/Users/firefly/Desktop/agent/broadcast/data/input/video/test.mp4",
  "clips": [
    {"title": "片段1", "start": 353.4, "end": 641.4},
    {"title": "片段2", "start": 3508.6, "end": 3598.96}
  ]
}
```

返回示例：

```json
{
  "outputs": [
    {
      "title": "片段1",
      "start": 353.4,
      "end": 641.4,
      "duration": 288.0,
      "file_path": "/abs/path/data/output/clips/xxx.mp4"
    }
  ]
}
```

## Dify 交付文件

1. 工作流 DSL：`AI 播客生产 Agent .yml`
2. 导入配置清单：`DIFY_IMPORT_CHECKLIST.md`

导入后必须修改：

1. Dify 环境变量：`DIFY_FILE_BASE_URL`、`BACKEND_CLIP_URL`
2. 模型供应商和 API Key
3. 本地 `video_path` 可访问性（需是剪辑服务所在机器的有效路径）

