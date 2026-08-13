import { Injectable } from '@angular/core';
import { ApiService } from './api.service';

/**
 * 音频朗读服务：
 * 1. 有 TTS audio_url 时用 <audio> 播放（真实 TTS 服务）
 * 2. 无 audio_url 时回退浏览器 SpeechSynthesis 朗读（免费、零依赖、支持英文语音）
 */
@Injectable({ providedIn: 'root' })
export class AudioService {
  private audio = new Audio();
  private synth: SpeechSynthesis | null =
    typeof window !== 'undefined' ? window.speechSynthesis : null;

  // 播放队列（全文朗读按角色逐句播放）
  private queue: { text: string; url?: string }[] = [];
  private queueIdx = -1;
  private onQueueIndex: ((i: number) => void) | null = null;
  private onQueueEnd: (() => void) | null = null;

  constructor(private api: ApiService) {}

  /** 播放文本：优先 TTS URL，回退浏览器朗读 */
  speak(text: string, url?: string, lang = 'en-US'): void {
    this.stopQueue();
    if (url) {
      this.stop();
      this.audio.src = this.api.mediaUrl(url);
      this.audio.play().catch(() => this.speakFallback(text, lang));
      return;
    }
    this.speakFallback(text, lang);
  }

  /**
   * 播放队列（全文朗读）：逐句播放，每句用各自的 TTS URL（不同角色不同音色自动切换）
   * @param onIndex 每句开始播放时回调（下标，用于高亮）
   * @param onEnd 全部播完回调
   */
  playQueue(items: { text: string; url?: string }[], onIndex?: (i: number) => void, onEnd?: () => void): void {
    this.stopQueue();
    this.stop();
    this.queue = items;
    this.onQueueIndex = onIndex || null;
    this.onQueueEnd = onEnd || null;
    this.audio.onended = () => this.nextInQueue();
    this.playQueueItem(0);
  }

  private playQueueItem(i: number): void {
    if (i >= this.queue.length) {
      this.endQueue();
      return;
    }
    this.queueIdx = i;
    this.onQueueIndex?.(i);
    const it = this.queue[i];
    if (it.url) {
      this.audio.src = this.api.mediaUrl(it.url);
      this.audio.play().catch(() => {
        // 音频播放失败：回退浏览器朗读（utterance 结束切下一句）
        this.speakFallback(it.text, 'en-US', () => this.nextInQueue());
      });
    } else {
      this.speakFallback(it.text, 'en-US', () => this.nextInQueue());
    }
  }

  private nextInQueue(): void {
    this.playQueueItem(this.queueIdx + 1);
  }

  private endQueue(): void {
    this.queue = [];
    this.onQueueIndex = null;
    this.onQueueEnd?.();
    this.onQueueEnd = null;
  }

  private stopQueue(): void {
    this.queue = [];
    this.onQueueIndex = null;
    this.onQueueEnd = null;
  }

  /** 浏览器原生朗读（无 TTS 服务时兜底） */
  private speakFallback(text: string, lang: string, onEnd?: () => void): void {
    if (!this.synth || !text) {
      onEnd?.();
      return;
    }
    this.synth.cancel();
    const u = new SpeechSynthesisUtterance(text);
    u.lang = lang;
    u.rate = 0.9; // 稍慢，适合学习
    u.pitch = 1;
    if (onEnd) u.onend = () => onEnd();
    this.synth.speak(u);
  }

  stop(): void {
    this.audio.pause();
    this.audio.currentTime = 0;
    this.synth?.cancel();
  }

  /** 当前浏览器可用的英文语音列表（调试用） */
  voices(): SpeechSynthesisVoice[] {
    return this.synth?.getVoices() ?? [];
  }
}
