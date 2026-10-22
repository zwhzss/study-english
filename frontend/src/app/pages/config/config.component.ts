import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { timeout } from 'rxjs';
import { ApiService } from '../../services/api.service';
import { AIConfig, ProviderSetting } from '../../models/models';

// 连通超时：30 秒无响应视为连接失败（与后端一致）
const TEST_TIMEOUT_MS = 30000;

// 常用平台预设：选平台自动填 Base URL + 推荐模型
const PLATFORM_PRESETS: Record<string, { name: string; url: string; models: string[] }[]> = {
  ocr: [
    { name: '硅基流动', url: 'https://api.siliconflow.cn/v1', models: ['Qwen/Qwen3-VL-8B-Instruct', 'deepseek-ai/DeepSeek-OCR', 'Qwen/Qwen3-VL-235B-A22B-Instruct'] },
    { name: 'OpenAI', url: 'https://api.openai.com/v1', models: ['gpt-4o-mini', 'gpt-4o'] },
    { name: '阿里云百炼', url: 'https://dashscope.aliyuncs.com/api/v1', models: ['qwen-vl-plus', 'qwen-vl-max'] },
  ],
  llm: [
    { name: 'DeepSeek', url: 'https://api.deepseek.com', models: ['deepseek-v4-flash', 'deepseek-v4-pro'] },
    { name: '硅基流动', url: 'https://api.siliconflow.cn/v1', models: ['deepseek-ai/DeepSeek-V3', 'Qwen/Qwen2.5-72B-Instruct'] },
    { name: 'OpenAI', url: 'https://api.openai.com/v1', models: ['gpt-4o-mini', 'gpt-4o'] },
    { name: '智谱AI', url: 'https://open.bigmodel.cn/api/paas/v4', models: ['glm-4-flash', 'glm-4-plus'] },
    { name: '阿里云百炼', url: 'https://dashscope.aliyuncs.com/api/v1', models: ['qwen-plus', 'qwen-turbo'] },
  ],
  tts: [
    { name: 'Edge TTS（免费·微软在线）', url: 'edge://local', models: ['edge-tts'] },
    { name: '硅基流动', url: 'https://api.siliconflow.cn/v1', models: ['FunAudioLLM/CosyVoice2-0.5B', 'fishaudio/fish-speech-1.5'] },
    { name: 'OpenAI', url: 'https://api.openai.com/v1', models: ['tts-1', 'tts-1-hd'] },
    { name: '阿里云百炼', url: 'https://dashscope.aliyuncs.com/api/v1', models: ['cosyvoice-v1', 'sambert-zhichu-v1'] },
  ],
  image: [
    { name: '硅基流动', url: 'https://api.siliconflow.cn/v1', models: ['Kwai-Kolors/Kolors', 'Qwen/Qwen-Image'] },
    { name: 'OpenAI', url: 'https://api.openai.com/v1', models: ['gpt-image-1', 'dall-e-3'] },
    { name: '阿里云百炼', url: 'https://dashscope.aliyuncs.com/api/v1', models: ['wanx2.1-t2i-turbo'] },
    { name: '智谱AI', url: 'https://open.bigmodel.cn/api/paas/v4', models: ['cogview-3-flash'] },
  ],
  asr: [
    { name: '硅基流动', url: 'https://api.siliconflow.cn/v1', models: ['iic/SenseVoiceSmall', 'FunAudioLLM/SenseVoiceSmall'] },
    { name: 'OpenAI', url: 'https://api.openai.com/v1', models: ['whisper-1'] },
  ],
};

@Component({
  selector: 'app-config',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './config.component.html',
  styleUrls: ['./config.component.css'],
})
export class ConfigComponent implements OnInit {
  cfg: AIConfig | null = null;
  loading = true;
  error = '';

  // 每个服务：测试/保存状态（各自独立）
  testing: Record<string, boolean> = { llm: false, tts: false, image: false };
  saving: Record<string, boolean> = { llm: false, tts: false, image: false };
  testResult: Record<string, { ok: boolean; msg: string }> = {};
  savedMsg: Record<string, string> = { llm: '', tts: '', image: '' };
  ttsStatusInfo: { provider: string; model: string; edge_available: boolean; voice_label: string } | null = null;

  constructor(private api: ApiService) {}

  /** 加载当前 TTS 生效通道状态 */
  loadTTSStatus(): void {
    this.api.ttsStatus().subscribe({
      next: (res) => { this.ttsStatusInfo = res.data; },
      error: () => {},
    });
  }

  ngOnInit(): void {
    this.loadTTSStatus();
    this.api.getAIConfig().subscribe({
      next: (res) => {
        this.cfg = res.data;
        this.loading = false;
      },
      error: (e) => {
        this.error = '加载配置失败：' + (e.error?.error || e.message);
        this.loading = false;
      },
    });
  }

  providerLabel(p: ProviderSetting): string {
    return p.provider === 'mock' ? 'Mock（内置示例）' : 'OpenAI 兼容 API';
  }

  /** 某服务的平台预设列表 */
  platformsOf(type: string): { name: string; url: string; models: string[] }[] {
    return PLATFORM_PRESETS[type] || [];
  }

  /** 一键填充：选平台自动填 Base URL + 推荐模型 */
  fillPreset(type: 'llm' | 'tts' | 'image' | 'ocr' | 'asr', platformUrl: string): void {
    const p = this.cfg?.[type];
    if (!p) return;
    const preset = this.platformsOf(type).find((x) => x.url === platformUrl);
    if (!preset) return;
    if (type === 'tts' && platformUrl === 'edge://local') {
      // Edge TTS：微软在线服务，免费多音色，无需 Key/URL
      p.provider = 'edge';
      p.base_url = '';
      p.api_key = '';
      p.model = 'edge-tts';
      p.extra = '';
      this.savedMsg[type] = `已切换「${preset.name}」：免费多音色（男孩/女孩/教师/爷爷奶奶等），保存后立即生效`;
      return;
    }
    p.provider = 'openai';
    p.base_url = preset.url;
    p.model = preset.models[0]; // 推荐模型（第一个）
    this.savedMsg[type] = `已填充「${preset.name}」：${preset.url} / ${p.model}，填入 API Key 后保存即可`;
  }

  /** 当前 LLM 平台的可用模型列表（如 DeepSeek: flash/pro；其他平台返回空则用自由输入） */
  llmModelsOf(): string[] {
    if (!this.cfg) return [];
    const preset = this.platformsOf('llm').find((x) => x.url === this.cfg!.llm.base_url);
    return preset?.models || [];
  }

  /** 切换并激活 LLM 模型 */
  activateLLMModel(m: string): void {
    if (!this.cfg) return;
    this.cfg.llm.model = m;
    this.savedMsg['llm'] = `已激活模型：${m}（保存后生效）`;
  }

  /** 测试连接：使用画面当前输入的值（非已保存/环境变量），30 秒超时提示 */
  test(type: 'llm' | 'tts' | 'image' | 'ocr' | 'asr'): void {
    if (!this.cfg) return;
    this.testing[type] = true;
    this.testResult[type] = { ok: false, msg: `测试中…（最长 ${TEST_TIMEOUT_MS / 1000} 秒）` };
    this.api.testAIConfig(type, this.cfg[type]).pipe(timeout(TEST_TIMEOUT_MS)).subscribe({
      next: (res) => {
        this.testResult[type] = { ok: true, msg: res.message };
        this.testing[type] = false;
      },
      error: (e) => {
        const timedOut = e?.name === 'TimeoutError' || e?.message?.includes('Timeout');
        this.testResult[type] = {
          ok: false,
          msg: timedOut
            ? `连接超时（${TEST_TIMEOUT_MS / 1000} 秒无响应），请检查 Base URL / API Key / 网络`
            : (e.error?.error || e.error?.message || e.message || '测试失败'),
        };
        this.testing[type] = false;
      },
    });
  }

  /** 保存配置（Key 未修改时传脱敏值，后端自动保留原 Key）——每卡片单独保存 */
  save(type: 'llm' | 'tts' | 'image' | 'ocr' | 'asr'): void {
    if (!this.cfg) return;
    this.saving[type] = true;
    this.savedMsg[type] = '';
    this.error = '';
    this.api.saveAIConfig(this.cfg).subscribe({
      next: (res) => {
        this.saving[type] = false;
        this.savedMsg[type] = res.data?.message ?? '已保存';
        // 保存成功后重新加载（拿到脱敏后的最新 key 显示）
        this.api.getAIConfig().subscribe({
          next: (r) => { this.cfg = r.data; },
          error: () => {},
        });
      },
      error: (e) => {
        this.saving[type] = false;
        this.error = '保存失败：' + (e.error?.error || e.message);
      },
    });
  }
}
