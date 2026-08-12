import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../services/api.service';
import { PronunciationRecord } from '../../models/models';

@Component({
  selector: 'app-pronunciation',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './pronunciation.component.html',
  styleUrls: ['./pronunciation.component.css'],
})
export class PronunciationComponent implements OnInit {
  // 录音
  recording = false;
  recSupported = true;
  title = '';
  targetText = '';
  recognizedText = '';
  durationMs = 0;
  audioBase64 = '';
  audioUrl = '';

  // 评测结果
  result: PronunciationRecord | null = null;
  history: PronunciationRecord[] = [];
  submitting = false;
  error = '';

  // 底层
  private mediaRecorder: MediaRecorder | null = null;
  private chunks: Blob[] = [];
  private recStart = 0;
  private speechRec: any = null;
  private stream: MediaStream | null = null;
  playingUrl = '';

  /** 分数颜色：≥85 绿、≥60 橙、否则红 */
  scoreColor(v: number): string {
    if (v >= 85) return '#188038';
    if (v >= 60) return '#e37400';
    return '#d93025';
  }

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    const w = window as any;
    this.recSupported = !!(w.MediaRecorder && (w.SpeechRecognition || w.webkitSpeechRecognition));
    this.loadHistory();
  }

  loadHistory(): void {
    this.api.listPronunciations().subscribe({
      next: (res) => (this.history = res.data),
      error: () => {},
    });
  }

  /** 开始录音：MediaRecorder 录原声 + SpeechRecognition 同步识别 */
  async startRecording(): Promise<void> {
    this.error = '';
    this.result = null;
    this.recognizedText = '';
    this.audioBase64 = '';
    this.audioUrl = '';
    try {
      this.stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      this.mediaRecorder = new MediaRecorder(this.stream, { mimeType: 'audio/webm' });
      this.chunks = [];
      this.mediaRecorder.ondataavailable = (e) => { if (e.data.size > 0) this.chunks.push(e.data); };
      this.mediaRecorder.onstop = () => this.finalizeAudio();
      this.mediaRecorder.start();
      this.recStart = Date.now();
      this.recording = true;

      // 同步语音识别（转文字，用于评分对比）
      const w = window as any;
      const Ctor = w.SpeechRecognition || w.webkitSpeechRecognition;
      this.speechRec = new Ctor();
      this.speechRec.lang = 'en-US';
      this.speechRec.interimResults = true;
      this.speechRec.continuous = true;
      this.speechRec.onresult = (e: any) => {
        let text = '';
        for (let i = 0; i < e.results.length; i++) text += e.results[i][0].transcript;
        this.recognizedText = text.trim();
      };
      this.speechRec.onend = () => {
        // 识别结束（停止时触发），保留已识别文本
      };
      try { this.speechRec.start(); } catch { /* ignore */ }
    } catch {
      this.error = '无法访问麦克风，请允许浏览器麦克风权限';
      this.recording = false;
    }
  }

  /** 停止录音：提交识别文本 + 生成音频 blob */
  stopRecording(): void {
    if (!this.mediaRecorder) return;
    this.mediaRecorder.stop();
    this.durationMs = Date.now() - this.recStart;
    this.recording = false;
    // 停止语音识别（提交最终文本）
    try { this.speechRec?.stop(); } catch { /* ignore */ }
    this.stream?.getTracks().forEach((t) => t.stop());
  }

  private finalizeAudio(): void {
    const blob = new Blob(this.chunks, { type: 'audio/webm' });
    this.audioUrl = URL.createObjectURL(blob);
    const reader = new FileReader();
    reader.onload = () => {
      this.audioBase64 = (reader.result as string).split(',')[1] || '';
    };
    reader.readAsDataURL(blob);
  }

  /** 提交评测 */
  submit(): void {
    if (!this.targetText.trim()) { this.error = '请填写目标文本（要朗读的句子）'; return; }
    if (!this.recognizedText.trim()) { this.error = '未识别到语音，请重新录音或检查麦克风'; return; }
    this.submitting = true;
    this.error = '';
    this.api.evaluatePronunciation({
      title: this.title, target_text: this.targetText,
      recognized_text: this.recognizedText, duration_ms: this.durationMs,
      audio_base64: this.audioBase64,
    }).subscribe({
      next: (res) => {
        this.submitting = false;
        this.result = res.data;
        this.loadHistory();
      },
      error: (e) => {
        this.submitting = false;
        this.error = '评测失败：' + (e.error?.error || e.message);
      },
    });
  }

  /** 播放录音（新录的或历史） */
  playAudio(url: string): void {
    if (this.playingUrl === url) { this.playingUrl = ''; return; }
    this.playingUrl = url;
  }

  /** 把历史目标文本带入评测（复练） */
  reuse(rec: PronunciationRecord): void {
    this.targetText = rec.target_text;
    this.result = null;
    this.recognizedText = '';
    this.audioBase64 = '';
    this.audioUrl = '';
  }

  deleteRec(rec: PronunciationRecord): void {
    if (!confirm('删除该评测记录（含录音）？')) return;
    this.api.deletePronunciation(rec.id).subscribe({
      next: () => this.loadHistory(),
      error: () => {},
    });
  }
}
