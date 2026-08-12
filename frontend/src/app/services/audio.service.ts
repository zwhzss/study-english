import { Injectable } from '@angular/core';

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

  /** 播放文本：优先 TTS URL，回退浏览器朗读 */
  speak(text: string, url?: string, lang = 'en-US'): void {
    if (url) {
      this.stop();
      this.audio.src = url;
      this.audio.play().catch(() => this.speakFallback(text, lang));
      return;
    }
    this.speakFallback(text, lang);
  }

  /** 浏览器原生朗读（无 TTS 服务时兜底） */
  private speakFallback(text: string, lang: string): void {
    if (!this.synth || !text) return;
    this.synth.cancel();
    const u = new SpeechSynthesisUtterance(text);
    u.lang = lang;
    u.rate = 0.9; // 稍慢，适合学习
    u.pitch = 1;
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
