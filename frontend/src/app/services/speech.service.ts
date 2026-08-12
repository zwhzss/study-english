import { Injectable } from '@angular/core';

/**
 * 浏览器语音识别服务（Web Speech API）：
 * - 识别到完整句子 → 立即结束并发送（不等静音检测，保证"说完即发"）
 * - 15 秒超时兜底：无论任何情况都强制结束（保证按钮恢复麦克风）
 * - 会话代次（generation）机制：旧会话异步回调不干扰新会话
 */
@Injectable({ providedIn: 'root' })
export class SpeechService {
  private rec: any = null;
  private running = false;
  private gen = 0; // 会话代次
  private timers = new Map<number, any>(); // gen → 超时 timer

  // 录音超时（毫秒）：无结束信号时强制停止，恢复按钮
  private static readonly REC_TIMEOUT_MS = 15000;

  supported(): boolean {
    return !!this.recImpl();
  }

  private recImpl(): any {
    const w = window as any;
    return w.SpeechRecognition || w.webkitSpeechRecognition || null;
  }

  get listening(): boolean {
    return this.running;
  }

  start(
    lang: string,
    onFinal: (text: string) => void,
    onInterim: (text: string) => void,
    onEnd: () => void
  ): void {
    const Ctor = this.recImpl();
    if (!Ctor) return;
    this.abort();

    const gen = ++this.gen;
    const rec = new Ctor();
    rec.lang = lang || 'en-US';
    rec.interimResults = true;
    rec.continuous = false;
    rec.maxAlternatives = 1;

    let finalReceived = false;
    rec.onresult = (e: any) => {
      if (gen !== this.gen) return;
      let interim = '';
      let final = '';
      for (let i = e.resultIndex; i < e.results.length; i++) {
        const r = e.results[i];
        if (r.isFinal) final += r[0].transcript;
        else interim += r[0].transcript;
      }
      if (final) {
        // 识别到完整句：立即发送并结束（不等静音检测）
        finalReceived = true;
        onFinal(final);
        try {
          rec.stop();
        } catch {
          /* ignore */
        }
      } else if (interim) {
        onInterim(interim);
      }
    };
    rec.onerror = () => this.finish(gen, onEnd);
    rec.onend = () => this.finish(gen, onEnd);

    this.rec = rec;
    this.running = true;
    // 超时兜底：无论是否识别到，15 秒强制结束（恢复按钮）
    const timer = setTimeout(() => {
      if (gen !== this.gen) return;
      try {
        rec.stop();
      } catch {
        /* ignore */
      }
      // 未识别到任何最终句，直接结束（恢复按钮）
      if (!finalReceived) this.finish(gen, onEnd);
    }, SpeechService.REC_TIMEOUT_MS);
    this.timers.set(gen, timer);

    try {
      rec.start();
    } catch {
      this.running = false;
      this.finish(gen, onEnd);
    }
  }

  /** 主动停止（手动点红色按钮）：提交最终结果 */
  stop(): void {
    if (this.rec) {
      const r = this.rec;
      try {
        r.stop();
      } catch {
        /* ignore */
      }
    }
  }

  private abort(): void {
    this.gen++;
    this.clearTimer(this.gen);
    if (this.rec) {
      const old = this.rec;
      this.rec = null;
      this.running = false;
      try {
        old.stop();
      } catch {
        /* ignore */
      }
    }
  }

  private finish(gen: number, onEnd: () => void): void {
    if (gen !== this.gen) return;
    this.clearTimer(gen);
    this.running = false;
    this.rec = null;
    onEnd?.();
  }

  private clearTimer(gen: number): void {
    const t = this.timers.get(gen);
    if (t) {
      clearTimeout(t);
      this.timers.delete(gen);
    }
  }
}
