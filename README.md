# AI 英语课堂（AI English Classroom）

基于《最终系统架构.txt》实现的全栈 MVP：**教材同步目录初始化 + 按 教材名称/年级/单元 从大模型拉取图文并茂、多角色对话、支持语音朗读的课时内容 + 学生端 AI 实景对话（语音 + SSE 流式）**。

- 后端：Go + Gin + GORM + SQLite（纯 Go 驱动，免 CGO，WAL 模式）
- 前端：Angular 19（standalone + PWA 骨架）
- AI：LLM Provider 可插拔（默认 DeepSeek，OpenAI 兼容协议；未配置 Key 自动 mock 兜底）
- **SSE 流式对话**：AI 回复逐字输出（打字机效果），DeepSeek stream 模式 / mock 分片模拟
- **AI 绘图配图**：LLM 输出绘图提示词 → 调 OpenAI 兼容绘图 API（OpenAI / 硅基流动 FLUX）→ 本地缓存 `/media/images/`；无 Key 自动回退模型配图
- TTS：OpenAI 兼容 `/audio/speech`（可对接 OpenAI/硅基流动），本地缓存音频；无 Key 时前端回退浏览器 SpeechSynthesis
- 语音输入：浏览器 Web Speech API（SpeechRecognition，Chrome/Edge）——内置端点检测（VAD）+ 语音转文本，无需外部 ASR

## 快速启动

```bash
# 1. 构建前端（输出到 frontend/dist/frontend/browser）
cd frontend && npm install && npx ng build && cd ..

# 2. 启动后端（Gin 自动托管前端静态文件，单端口）
cd server && go build -o study-english-server.exe . && ./study-english-server.exe
#   默认端口 8090；可用 APP_PORT 覆盖

# 3. 打开浏览器
#   http://localhost:8090
```

**开发模式（前端热部署 HMR）**：改前端代码自动热更新，无需重新构建：
```bash
# Windows：双击 start-dev.bat（一键起 后端8090 + 前端4200）
# 或手动：
cd server && go run . &          # 终端1：后端 8090
cd frontend && npx ng serve      # 终端2：前端 4200（HMR 热更新）
# 前端 /api 请求由 proxy.conf.json 自动代理到后端 8090
# 浏览器打开 http://localhost:4200，保存 .ts/.html/.css 即热更新
```

体验路径：
1. **教材同步**：左侧「➕ 同步新教材」输入 `人教版七年级英语上册` → 目录树自动初始化（5 单元 10 课时）
2. **内容拉取**：点课时 →「🤖 从大模型拉取本课内容」→ 图文并茂 + 多角色对话气泡 + **SVG 示意图（LLM 生成、前端零成本渲染）** + 词汇/语法/练习，点击气泡即朗读
3. **AI 实景对话**：课时页点「🎭 与 AI 角色对话」→ 选择扮演角色 → 点 🎤 说英文（静音自动发送，AI 流式打字机回复并朗读）；首页还有「自由对话」入口
4. **能力画像**：左侧「📊 能力画像」→ Plotly 雷达图展示 听/说/读/写 四维能力值 + 最薄弱维度 + 学习建议（能力型，不记考试分）
5. **模型配置**：左侧「⚙️ 模型配置」→ 页面配置 LLM/TTS/绘图 的 Base URL / API Key / 模型，**测试连接 + 保存立即生效**（无需重启）；Key 脱敏显示；**「常用平台一键填充」**（DeepSeek/硅基流动/OpenAI/智谱/阿里云，选平台自动填 Base URL + 推荐模型）
6. **教材同步下拉**：创建教材用下拉选择（常用教材预设），出版社/年级自动带出，消除手工输入歧义；**内置真实目录**：人教版七上（Starter 1-3 + Unit 1-7 共 10 单元 20 课时）、外研版七上（Module 1-10 共 20 课时）、牛津译林七上（Starter 1-4 + Unit 1-8 共 20 课时）、新概念第一册（Lesson 1-16）
7. **课文导入（版权合规）**：课时「📥 导入课文」→ 粘贴文本或上传 PDF → 课文原文入库展示（source=imported，标注"已导入 · 非 AI 生成"），可朗读；**AI 创作**内容为独立拓展（无版权问题），可「🔄 重新生成」
7. **未配置提示**：未设置大模型 API Key（且未显式选 Mock）时，拉取**新**课时内容会提示「请到模型配置页面设置」；已拉取过的课时走缓存不受影响；显式选「Mock」可继续体验示例内容

## 配置（环境变量）

| 变量 | 默认 | 说明 |
|---|---|---|
| `APP_PORT` | `8090` | 服务端口 |
| `DB_PATH` | `data/study-english.db` | SQLite 文件 |
| `LLM_API_KEY` | 空 | 大模型 Key；**为空则使用内置 mock 内容** |
| `LLM_BASE_URL` | `https://api.deepseek.com` | OpenAI 兼容地址 |
| `LLM_MODEL` | `deepseek-chat` | 模型名 |
| `TTS_API_KEY` | 空 | TTS Key；为空则前端用浏览器朗读 |
| `TTS_BASE_URL` | `https://api.openai.com/v1` | OpenAI 兼容 TTS 地址（硅基流动: `https://api.siliconflow.cn/v1`） |
| `TTS_MODEL` | `tts-1` | TTS 模型（硅基流动: `FunAudioLLM/CosyVoice2-0.5B` 等） |
| `TTS_VOICE` | 空 | 默认音色（空则按角色自动映射：waiter→男声、customer→女声…） |
| `IMAGE_API_KEY` | 空 | 绘图 Key；为空则配图回退模型给出的 URL |
| `IMAGE_BASE_URL` | `https://api.siliconflow.cn/v1` | OpenAI 兼容绘图地址 |
| `IMAGE_MODEL` | `black-forest-labs/FLUX.1-schnell` | 绘图模型（OpenAI 用 `gpt-image-1`） |
| `IMAGE_SIZE` | `1024x1024` | 图片尺寸 |
| `MOCK_CONTENT` | `0` | `1` 强制 mock |
| `DIST_DIR` | `../frontend/dist/frontend/browser` | 前端产物目录 |

真实大模型：`LLM_API_KEY=sk-xxx ./study-english-server.exe`（生成任意教材/年级/单元的真实多角色内容 + 真实对话）。
真实语音：`TTS_API_KEY=sk-xxx TTS_BASE_URL=https://api.siliconflow.cn/v1 TTS_MODEL=FunAudioLLM/CosyVoice2-0.5B ./study-english-server.exe`
真实配图：`IMAGE_API_KEY=sk-xxx ./study-english-server.exe`（硅基流动 FLUX.1-schnell 默认）
验证连接：`curl http://localhost:8090/api/ai/test`

## 绘图：云端还是本地？

两种都支持，接口完全一致（OpenAI 兼容 `/images/generations`），主系统无感知切换。

- **云端（推荐，零部署）**：填 `IMAGE_API_KEY` 即可，默认走 `IMAGE_PROVIDER=auto`——有 Key 自动用云端 FLUX.1-schnell，无 Key 回退模型给出的配图 URL。
- **本地**：`local-image/` 是一个 OpenAI 兼容的本地绘图适配服务（默认 `http://localhost:17000`），把请求转发给本机的 `llama-cli` / ComfyUI 等图像后端。需要自备 GGUF 模型与对应推理框架，按你机器的显存自行调整并发（`local-image/main.go` 里默认限制 4 张）。
  - 注意：**llama.cpp 主线不支持 FLUX 图像生成**，需自行编译特殊分支或改用 ComfyUI。因此本项目日常使用建议直接走云端。
  - 启动本地服务后，设 `IMAGE_PROVIDER=openai` + `IMAGE_BASE_URL=http://localhost:17000/v1` 即可接入；去掉该环境变量就回云端。
- 生成的图片缓存在数据目录 `data/images/`，同提示词不重复生成。

## API

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 健康检查 |
| GET | `/api/ai/test` | 大模型连接测试（返回模式/耗时） |
| POST | `/api/textbooks` | 创建教材 + 初始化内置目录 |
| GET | `/api/textbooks` | 教材列表 |
| GET | `/api/textbooks/:id/catalog` | 目录树（年级→单元→课时） |
| POST | `/api/lessons/:id/pull` | 从大模型拉取课时内容（`{"force":true}` 重新生成覆盖） |
| GET | `/api/lessons/:id/content` | 查询课时内容 |
| POST | `/api/lessons/:id/import` | **导入课文**（JSON 粘贴文本 / multipart 上传 PDF，版权合规通道，纯 Go 解析免 CGO） |
| POST | `/api/dialogue/sessions` | 创建 AI 实景对话会话（选扮演角色） |
| GET | `/api/dialogue/sessions/:id/messages` | 会话历史 |
| POST | `/api/dialogue/sessions/:id/messages` | 发送消息，AI 角色回复 |
| POST | `/api/dialogue/sessions/:id/messages/stream` | **SSE 流式发送**（事件：`delta` 增量 → `done` 完整消息 / `error`） |
| GET | `/api/ability-profile` | **能力画像**（听/说/读/写四维 + 最弱维度 + 建议，雷达图数据源） |
| GET | `/api/textbooks/meta` | **常用教材预设**（下拉列表：教材/出版社/年级阶段联动） |
| GET | `/api/config/ai` | **模型配置读取**（API Key 脱敏） |
| POST | `/api/config/ai` | **保存模型配置**（保存后立即生效，无需重启） |
| POST | `/api/config/ai/test` | **测试连接**（`{"type":"llm"/"tts"/"image"}`） |
| GET | `/media/audio/:file` | TTS 合成音频（本地缓存） |
| GET | `/media/images/:file` | AI 绘图生成配图（本地缓存） |

## 测试

```bash
cd server && go test ./...
```

覆盖：教材创建与目录初始化（人教版 5 单元 10 课时）、LLM 内容拉取（多角色 ≥2、词汇/配图非空）、重复拉取缓存命中、**并发 4 路拉取防重**、未生成内容报错、对话会话创建（角色过滤）、对话消息往返、非法角色、自由对话。

## 目录结构

```
server/
  main.go                   入口
  internal/config/          配置（环境变量）
  internal/model/           GORM 模型（Textbook/CatalogItem/LessonContent/DialogueSession/Message）
  internal/database/        SQLite 初始化（WAL + 连接池）
  internal/ai/              LLM Provider（DeepSeek + Mock，含 Chat/ChatJSON/Test）、TTS（OpenAI 兼容 + Mock）
  internal/service/         业务层（教材目录/内容拉取/实景对话）
  internal/handler/         Gin 路由 + 处理器 + 静态/音频托管
frontend/
  src/app/
    models/                 与后端对应的类型
    services/               API 封装、音频朗读（TTS + 浏览器回退）、语音识别（SpeechRecognition）
    pages/textbooks/        教材页（目录树 + 图文 + 对话气泡 + 词汇/语法/练习）
    pages/dialogue/         AI 实景对话页（角色选择 + 语音对话 + AI 回复朗读）
```

## 许可证

本项目采用**个人学习与教育用途许可**，详见 [LICENSE](LICENSE)。要点：

- 允许：个人学习、研究、课堂/家庭自用，以及在此基础上修改和分享非商业副本。
- 禁止：任何形式的商业用途（销售、SaaS/托管、企业内使用、付费培训、集成进商业产品等），须事先取得作者书面授权。
- 本项目会调用第三方 AI 服务，产生的内容版权归相应服务条款约束。
- 教材课文、配套录音与图片版权归出版社所有，**不随本仓库分发**；相关素材需你自行合法获得。
