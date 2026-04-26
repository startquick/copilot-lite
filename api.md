# GrokPi API Reference

This document summarizes the relevant API for the Studio frontend (Prompt Lab, Image, Video) based on the current server implementation.

## 1. Base URL and Auth

| Item | Value |
| --- | --- |
| Local Base URL | `http://127.0.0.1:8080` |
| Health endpoint | `GET /health` or `GET /healthz` (no auth) |
| Main API endpoint | Prefix `/v1/*` |
| Auth for `/v1/*` | Header `Authorization: Bearer <API_KEY>` |
| Common auth errors | `401 invalid_api_key`, `429 rate_limit_exceeded`, `429 daily_limit_exceeded` |

Example header:

```http
Authorization: Bearer gf-xxxxxxxxxxxxxxxx
Content-Type: application/json
```

## 2. Endpoint Summary

| Method | Path | Auth | Function |
| --- | --- | --- | --- |
| GET | `/health` | No | Check server status |
| GET | `/v1/models` | API key | Get available models for the key |
| POST | `/v1/chat/completions` | API key | Chat, image generate/edit, video generate (depends on model) |
| GET | `/api/files/video/{name}` | No | Get cached video file from generated URL |

Notes:
- There is no separate endpoint specifically for image/video. Media generation is called via `POST /v1/chat/completions` using media models.
- The generated video URL is usually already an absolute URL to `/api/files/video/{name}`.

## 3. Available Models (Currently Live)

Here is the result of `GET /v1/models` in this environment when this document was created:

| Model ID | Category |
| --- | --- |
| `grok-3` | Chat |
| `grok-3-mini` | Chat |
| `grok-3-thinking` | Chat |
| `grok-4` | Chat |
| `grok-4-mini` | Chat |
| `grok-4-thinking` | Chat |
| `grok-4-heavy` | Chat |
| `grok-4.1-fast` | Chat |
| `grok-4.1-mini` | Chat |
| `grok-4.1-thinking` | Chat |
| `grok-4.1-expert` | Chat |
| `grok-4.20-beta` | Chat |
| `grok-imagine-1.0` | Image generate |
| `grok-imagine-1.0-fast` | Image generate (fast defaults) |
| `grok-imagine-1.0-edit` | Image edit |
| `grok-imagine-1.0-video` | Video generate |

Important notes:
- This list is dynamic according to the `token.basic_models` and `token.super_models` configuration.
- If the API key has a `model_whitelist`, the final result may be fewer.

## 4. GET /v1/models

### Request

```http
GET /v1/models HTTP/1.1
Authorization: Bearer gf-xxxxxxxxxxxxxxxx
```

### Success Response

```json
{
  "object": "list",
  "data": [
    {
      "id": "grok-imagine-1.0-video",
      "object": "model",
      "created": 1709251200,
      "owned_by": "xai"
    }
  ]
}
```

## 5. POST /v1/chat/completions (Main Endpoint)

## 5.1 Common Request Fields

| Field | Type | Required | Default | Description |
| --- | --- | --- | --- | --- |
| `model` | string | Yes | - | Target model |
| `messages` | array | Yes | - | Conversation history / multimodal blocks |
| `stream` | bool | No | Follows `app.stream` config | `true` for SSE stream |
| `temperature` | number | No | `0.8` | Range `0` to `2` |
| `top_p` | number | No | `0.95` | Range `0` to `1` |
| `max_tokens` | int | No | - | Token output limit for chat |
| `reasoning_effort` | string | No | - | `none|minimal|low|medium|high|xhigh` |
| `tools` | array | No | - | Tool definitions for tool-calling |
| `tool_choice` | string/object | No | - | `auto|required|none` or object function |
| `parallel_tool_calls` | bool | No | `true` | Parallel tool calls |
| `image_config` | object | No | Depends on model | Used by image models |
| `video_config` | object | No | Depends on model | Used by video models |

Important validations:
- `model` is required.
- `messages` cannot be empty.
- `message.content` cannot be null/empty.

## 5.2 messages Format (text + image)

### A. Simple text format

```json
{
  "role": "user",
  "content": "Create a cyberpunk neon poster"
}
```

### B. Multimodal blocks format (recommended for image edit/video reference)

```json
{
  "role": "user",
  "content": [
    { "type": "text", "text": "Change style to cinematic" },
    { "type": "image_url", "image_url": { "url": "data:image/png;base64,...." } }
  ]
}
```

Supported block rules on the `user` role:

| `type` | Description |
| --- | --- |
| `text` | Text prompt |
| `image_url` | Image URL / base64 data URI |
| `input_audio` | Audio input (validation block format available) |
| `file` | File input (validation block format available) |

For roles other than `user`, the supported block content is `text`.

## 6. Image API via Chat Completions

Use models:
- `grok-imagine-1.0`
- `grok-imagine-1.0-fast`
- `grok-imagine-1.0-edit`

### 6.1 image_config

| Field | Type | Default | Constraints |
| --- | --- | --- | --- |
| `n` | int | `1` | `1` to `10` |
| `size` | string | `1024x1024` | Only permitted values (see size table) |
| `response_format` | string | `b64_json` | Currently forced normalized to `b64_json` |
| `enable_nsfw` | bool | follows system | Optional NSFW flag |

### 6.2 Permitted Image Sizes

| Size |
| --- |
| `1024x1024` |
| `1024x1792` |
| `1792x1024` |
| `1280x720` |
| `720x1280` |

Stream notes for images:
- If `stream=true`, then `image_config.n` can only be `1` or `2`.

### 6.3 Specific for edit model

The `grok-imagine-1.0-edit` model requires at least 1 image from the `image_url` block in user messages.

## 6.4 Example generate image request

```json
{
  "model": "grok-imagine-1.0",
  "stream": false,
  "messages": [
    {
      "role": "user",
      "content": "A minimalist product photo of a smart watch on white background"
    }
  ],
  "image_config": {
    "n": 1,
    "size": "1024x1024",
    "response_format": "b64_json"
  }
}
```

## 6.5 Example edit image request

```json
{
  "model": "grok-imagine-1.0-edit",
  "stream": false,
  "messages": [
    {
      "role": "user",
      "content": [
        { "type": "text", "text": "Make this image warmer and cinematic" },
        { "type": "image_url", "image_url": { "url": "data:image/png;base64,...." } }
      ]
    }
  ],
  "image_config": {
    "n": 1,
    "size": "1792x1024"
  }
}
```

## 7. Video API via Chat Completions

Use model:
- `grok-imagine-1.0-video`

### 7.1 video_config

| Field | Type | Default | Constraints |
| --- | --- | --- | --- |
| `aspect_ratio` | string | `3:2` | See valid values below |
| `video_length` | int | `6` | `6` to `30` seconds |
| `resolution_name` | string | `480p` | `480p` or `720p` |
| `preset` | string | `custom` | `custom|fun|normal|spicy` |

### 7.2 Accepted Aspect Ratios

Can use the following ratios or size aliases:

| Accepted input | Normalized to |
| --- | --- |
| `16:9` or `1280x720` | `16:9` |
| `9:16` or `720x1280` | `9:16` |
| `3:2` or `1792x1024` | `3:2` |
| `2:3` or `1024x1792` | `2:3` |
| `1:1` or `1024x1024` | `1:1` |

### 7.3 Resolution + Ratio Mapping to Internal Size

The server calculates the internal size from the formula:
- height = `480` for `480p`
- height = `720` for `720p`
- width = `height * ratio_w / ratio_h` (integer)

| resolution_name | aspect_ratio | internal size |
| --- | --- | --- |
| `480p` | `16:9` | `853x480` |
| `480p` | `9:16` | `270x480` |
| `480p` | `3:2` | `720x480` |
| `480p` | `2:3` | `320x480` |
| `480p` | `1:1` | `480x480` |
| `720p` | `16:9` | `1280x720` |
| `720p` | `9:16` | `405x720` |
| `720p` | `3:2` | `1080x720` |
| `720p` | `2:3` | `480x720` |
| `720p` | `1:1` | `720x720` |

### 7.4 Supported Presets

| Preset |
| --- |
| `custom` |
| `fun` |
| `normal` |
| `spicy` |

### 7.5 Reference image for video

If there is an `image_url` block in the user message, the server uses the first image as the `reference_image`.

### 7.6 Example video request

```json
{
  "model": "grok-imagine-1.0-video",
  "stream": false,
  "messages": [
    {
      "role": "user",
      "content": [
        { "type": "text", "text": "A cinematic drone shot over tropical island at sunrise" }
      ]
    }
  ],
  "video_config": {
    "aspect_ratio": "16:9",
    "video_length": 10,
    "resolution_name": "720p",
    "preset": "normal"
  }
}
```

## 8. Response Format

### 8.1 Non-stream (`stream=false`)

OpenAI-compatible shape response:

```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1710000000,
  "model": "grok-imagine-1.0-video",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "[video](http://127.0.0.1:8080/api/files/video/xxx.mp4)"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0
  }
}
```

Media output notes:
- Images are typically returned as markdown images with a base64 data URI.
- Videos are returned as a markdown link `[video](...)`.

### 8.2 Stream (`stream=true`)

- Content-Type: SSE
- Format: `chat.completion.chunk`
- End of stream: `data: [DONE]`

## 9. Important Error Codes

| HTTP | code | When it occurs |
| --- | --- | --- |
| 400 | `invalid_json` | Broken JSON request |
| 400 | `missing_model` | `model` field is empty |
| 400 | `invalid_messages` | messages empty/invalid |
| 400 | `invalid_temperature` | outside `0..2` |
| 400 | `invalid_top_p` | outside `0..1` |
| 400 | `invalid_image_config` | invalid image_config |
| 400 | `invalid_video_config` | invalid video_config |
| 400 | `missing_prompt` | empty prompt for image/video |
| 400 | `missing_image` | image edit without image_url |
| 401 | `invalid_api_key` | API key empty/invalid/inactive/expired |
| 403 | `model_not_allowed` | model is not in API key whitelist |
| 403 | `media_generation_disabled` | media feature disabled by admin |
| 404 | `model_not_found` | model is not in model pool config |
| 429 | `rate_limit_exceeded` | per-minute API key limit exceeded |
| 429 | `daily_limit_exceeded` | daily API key quota exhausted |

## 10. Studio Frontend Integration Notes

- Always call `GET /v1/models` on app load to sync models in real-time.
- For the video tab, filter for the `grok-imagine-1.0-video` model.
- For the image tab, use `grok-imagine-1.0`, `grok-imagine-1.0-fast`, `grok-imagine-1.0-edit`.
- Store the API key per user/session, do not hardcode in the frontend source.
- Display error messages based on `error.code` so the UX is clearer.
