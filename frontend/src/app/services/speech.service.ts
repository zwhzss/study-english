import { Injectable } from '@angular/core';
import { ApiService } from './api.service';
import { timeout } from 'rxjs';

/**
 * 语音输入服务（WebView2/浏览器通用）：
 * 1. MediaRecorder 录制原声音频（不把原声发给 AI 对话，仅本地转文字）
 * 2. 停止后把音频发给后端 ASR（SenseVoice）识别为英文文本
 */
@Injectable({ providedIn: 'root' })
export class SpeechService {
  recording = false;
  private mediaRecorder: MediaRecorder | null = null;
  private chunks: Blob[] = [];
  private stream: MediaStream | null = null;
  private onResult: ((text: string) => void) | null = null;
  private onError: ((msg: string) => void) | null = null;
  private onState: ((recording: boolean) => void) | null = null;

  constructor(private api: ApiService) {}

  /** 开始录音（识别文本通过 onResult 回调返回） */
  async start(opts: {
    onResult: (text: string) => void;
    onError?: (msg: string) => void;
    onState?: (recording: boolean) => void;
  }): Promise<void> {
    this.onResult = opts.onResult;
    this.onError = opts.onError || null;
    this.onState = opts.onState || null;
    try {
      this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    } catch {
      this.fail('无法访问麦克风，请允许麦克风权限');
      return;
    }
    this.chunks = [];
    try {
      this.mediaRecorder = new MediaRecorder(this.stream, {
        mimeType: 'audio/webm;codecs=opus',
      });
    } catch {
      try {
        this.mediaRecorder = new MediaRecorder(this.stream);
      } catch {
        this.stream.getTracks().forEach((t) => t.stop());
        this.fail('当前环境不支持录音');
        return;
      }
    }
    this.mediaRecorder.ondataavailable = (e) => {
      if (e.data.size > 0) this.chunks.push(e.data);
    };
    this.mediaRecorder.start();
    this.recording = true;
    this.onState?.(true);
  }

  /** 停止录音并发送给后端 ASR 识别 */
  stop(): void {
    if (!this.mediaRecorder) return;
    this.mediaRecorder.onstop = () => this.transcribe();
    this.mediaRecorder.stop();
    this.stream?.getTracks().forEach((t) => t.stop());
    this.recording = false;
    this.onState?.(false);
  }

  /** 放弃当前录音（不识别） */
  cancel(): void {
    if (this.mediaRecorder && this.mediaRecorder.state !== 'inactive') {
      try { this.mediaRecorder.stop(); } catch { /* ignore */ }
    }
    this.stream?.getTracks().forEach((t) => t.stop());
    this.recording = false;
    this.onState?.(false);
    this.chunks = [];
  }

  private transcribe(): void {
    const blob = new Blob(this.chunks, { type: 'audio/webm' });
    if (blob.size < 200) {
      this.fail('录音太短，请再说一遍');
      return;
    }
    this.api.transcribeASR(blob, 'webm').pipe(timeout(30000)).subscribe({
      next: (res) => {
        const text = (res.text || '').trim();
        if (!text) {
          this.fail('未识别到语音内容，请重新录音');
          return;
        }
        this.onResult?.(text);
      },
      error: (e) => this.fail(e.error?.error || e.message || '语音识别失败'),
    });
  }

  private fail(msg: string): void {
    this.recording = false;
    this.onState?.(false);
    this.onError?.(msg);
  }
}
