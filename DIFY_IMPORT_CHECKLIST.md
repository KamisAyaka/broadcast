# Dify 导入后配置清单

适用文件：`AI 播客生产 Agent .yml`

## 1. 导入方式

1. 在 Dify 里创建应用时选择 `Import DSL`。
2. 导入 `AI 播客生产 Agent .yml`。

## 2. 必改环境变量

导入后在 Workflow 的 Environment Variables 中检查并修改：

1. `DIFY_FILE_BASE_URL`
   - 含义：工作流内 HTTP 节点读取上传文件时的 Dify 文件服务地址前缀
   - 常见值：
     - Docker 内网：`http://nginx`
     - 非 Docker 部署：`http://127.0.0.1` 或你的 Dify 域名
2. `BACKEND_CLIP_URL`
   - 含义：本地/远端剪辑服务接口地址
   - 示例：`http://host.docker.internal:8088/v1/video/clip`
3. `Authorization`
   - 当前流程节点未使用，可留空或删除。

## 3. 必改模型与密钥

1. 检查 LLM 节点模型是否可用（例如 `gpt-5-chat-latest`）。
2. 在 Dify 的模型供应商设置里配置自己的 API Key。

## 4. 必查输入输出映射

1. `video_path` 必须来自 transcript JSON 的 `source_file` 字段。
2. 传给剪辑接口的 `clips` 必须是数组，不是字符串。
3. 最后 HTTP 节点 body 结构应为：

```json
{
  "video_path": "{{#...video_path#}}",
  "clips": {{#...clips#}}
}
```

## 5. 常见错误排查

1. `400 invalid request`
   - 重点检查 `clips` 是否被当成字符串传递。
2. `video_path not found`
   - 重点检查 `source_file` 是否是目标机器可访问的绝对路径。
3. `Reached maximum retries for URL .../files/...`
   - 重点检查 `DIFY_FILE_BASE_URL` 是否可从 Dify 运行环境访问。

## 6. 建议的移植步骤

1. 先在目标环境单独测试剪辑服务 `/v1/video/clip`。
2. 再在 Dify 中测试“读取 transcript 文件”节点。
3. 最后跑整条工作流并验证输出片段路径。
