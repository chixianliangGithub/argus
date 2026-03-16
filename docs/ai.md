# AI 诊断配置

当前系统默认使用 Mock 模型，不需要任何 API Key。要接入真实大模型，请在 `configs/config.yaml` 配置 `ai` 段。

## OpenAI

```yaml
ai:
  provider: openai
  api_key: "YOUR_OPENAI_API_KEY"
  base_url: "https://api.openai.com/v1"
  model: "gpt-4o-mini"
  timeout_seconds: 30
```

## DeepSeek（OpenAI 兼容）

```yaml
ai:
  provider: deepseek
  api_key: "YOUR_DEEPSEEK_API_KEY"
  base_url: "https://api.deepseek.com/v1"
  model: "deepseek-chat"
  timeout_seconds: 30
```

## 注意事项

- `base_url` 需要是 OpenAI 兼容的 API 根路径（末尾不要带 `/chat/completions`）。
- 不要把真实 key 提交到代码仓库。
