import { HttpClient } from '@angular/common/http';
import { Injectable, NgZone } from '@angular/core';
import { Observable, from, map, mergeMap, of, throwError } from 'rxjs';
import { ApiResponse, AbilityProfile, AIConfig, CatalogNode, DialogueMessage, DialogueSession, ImportedContent, LessonContent, PronunciationRecord, ProviderSetting, Textbook, TextbookMeta } from '../models/models';

/**
 * ApiService：双通道传输
 * - 桌面版（Wails）：直接调用 Go 绑定方法（window.go.main.App.*），无 HTTP、无 data 包装
 * - 浏览器版（HTTP）：走 /api（Gin 托管或 ng serve 代理），返回体带 data 包装
 */
@Injectable({ providedIn: 'root' })
export class ApiService {
  private base = '/api';

  getBase(): string {
    return this.base;
  }

  constructor(private http: HttpClient, private ngZone: NgZone) {}

  /** 是否运行在 Wails 桌面环境 */
  inWails(): boolean {
    return !!(window as any).go?.main?.App;
  }

  /**
   * 调用 Wails 绑定方法（返回 Promise）。
   * 关键：Wails 的 Promise 回调发生在 Angular zone 外，必须用 ngZone.run 包回，
   * 否则组件字段更新（loading/recognizing/数据）不会触发变更检测 → 界面不刷新（经典坑）
   */
  private wails<T>(name: string, ...args: any[]): Observable<T> {
    const app = (window as any).go?.main?.App;
    if (!app?.[name]) {
      return throwError(() => new Error(`Wails 绑定方法 ${name} 不可用`));
    }
    return new Observable<T>((sub) => {
      const p = app[name](...args) as Promise<T>;
      p.then(
        (v) => this.ngZone.run(() => { sub.next(v); sub.complete(); }),
        (e) => this.ngZone.run(() => sub.error(e instanceof Error ? e : new Error(String(e))))
      );
    });
  }

  /** 媒体相对路径 → 可播放 URL（Wails 由 asset Handler 提供 /media/，浏览器走 HTTP 同路径） */
  mediaUrl(url: string): string {
    return url;
  }

  /** 语音识别：音频 blob → 文本（后端 ASR：SenseVoice） */
  transcribeASR(audio: Blob, format = 'webm'): Observable<{ text: string }> {
    if (this.inWails()) {
      return from(audio.arrayBuffer()).pipe(
        map((buf) => {
          const bytes = new Uint8Array(buf);
          let bin = '';
          for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
          return btoa(bin);
        }),
        mergeMap((b64) => this.wails<string>('Transcribe', b64, format).pipe(map((text) => ({ text }))))
      );
    }
    const fd = new FormData();
    fd.append('file', audio, 'audio.' + format);
    fd.append('format', format);
    return this.http.post<{ text: string }>(`${this.base}/asr/transcribe`, fd);
  }

  // ========== 教材 ==========

  listTextbooks(): Observable<ApiResponse<Textbook[]>> {
    if (this.inWails()) return this.wails<Textbook[]>('ListTextbooks').pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<Textbook[]>>(`${this.base}/textbooks`);
  }

  createTextbook(body: { name: string; publisher?: string; stage?: string; grade?: string }): Observable<ApiResponse<Textbook>> {
    if (this.inWails()) return this.wails<Textbook>('CreateTextbook', body.name, body.publisher || '', body.stage || '', body.grade || '').pipe(map((d) => ({ data: d })));
    return this.http.post<ApiResponse<Textbook>>(`${this.base}/textbooks`, body);
  }

  deleteTextbook(id: number): Observable<ApiResponse<{ ok: boolean; message: string }>> {
    if (this.inWails()) return this.wails<void>('DeleteTextbook', id).pipe(map(() => ({ data: { ok: true, message: '已删除' } })));
    return this.http.delete<ApiResponse<{ ok: boolean; message: string }>>(`${this.base}/textbooks/${id}`);
  }

  reorderTextbooks(ids: number[]): Observable<ApiResponse<{ ok: boolean; message: string }>> {
    if (this.inWails()) return this.wails<void>('ReorderTextbooks', ids).pipe(map(() => ({ data: { ok: true, message: '已排序' } })));
    return this.http.put<ApiResponse<{ ok: boolean; message: string }>>(`${this.base}/textbooks/order`, { ids });
  }

  getCatalog(textbookId: number): Observable<ApiResponse<CatalogNode[]>> {
    if (this.inWails()) return this.wails<CatalogNode[]>('GetCatalog', textbookId).pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<CatalogNode[]>>(`${this.base}/textbooks/${textbookId}/catalog`);
  }

  reorderCatalog(textbookId: number, ids: number[]): Observable<ApiResponse<{ ok: boolean; message: string }>> {
    if (this.inWails()) return this.wails<void>('ReorderCatalog', textbookId, ids).pipe(map(() => ({ data: { ok: true, message: '已排序' } })));
    return this.http.put<ApiResponse<{ ok: boolean; message: string }>>(`${this.base}/textbooks/${textbookId}/catalog/order`, { ids });
  }

  /** 外链图片批量下载到本地（历史课文图片永久化） */
  migrateImages(): Observable<ApiResponse<{ lessons: number; downloaded: number; failed: number }>> {
    if (this.inWails()) return this.wails<{ lessons: number; downloaded: number; failed: number }>('MigrateImages').pipe(map((d) => ({ data: d })));
    return this.http.post<ApiResponse<{ lessons: number; downloaded: number; failed: number }>>(`${this.base}/images/migrate`, {});
  }

  getTextbookMeta(): Observable<ApiResponse<TextbookMeta>> {
    if (this.inWails()) return this.wails<TextbookMeta>('TextbookMeta').pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<TextbookMeta>>(`${this.base}/textbooks/meta`);
  }

  // ========== 课时内容（AI 创作 / 导入） ==========

  /** 异步拉取课时内容（后台任务，返回 task_id） */
  pullLessonContent(lessonId: number, force = false): Observable<{ task_id: string; status: string; message: string }> {
    if (this.inWails()) return this.wails<string>('StartPullLesson', lessonId, force).pipe(map((id) => ({ task_id: id, status: 'running', message: 'AI 生成已开始' })));
    return this.http.post<{ task_id: string; status: string; message: string }>(`${this.base}/lessons/${lessonId}/pull`, { force });
  }

  /** 异步任务状态（done 时含结果 content） */
  getTask(taskId: string): Observable<ApiResponse<{ id: string; status: string; error?: string; content?: LessonContent }>> {
    if (this.inWails()) return this.wails<{ id: string; status: string; error?: string; content?: LessonContent }>('GetTask', taskId).pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<{ id: string; status: string; error?: string; content?: LessonContent }>>(`${this.base}/tasks/${taskId}`);
  }

  getLessonContent(lessonId: number, kind = 'ai'): Observable<ApiResponse<LessonContent | ImportedContent>> {
    if (this.inWails()) return this.wails<LessonContent | ImportedContent>('GetLessonContent', lessonId, kind).pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<LessonContent | ImportedContent>>(`${this.base}/lessons/${lessonId}/content?kind=${kind}`);
  }

  importLessonText(lessonId: number, title: string, text: string): Observable<ApiResponse<ImportedContent>> {
    if (this.inWails()) return this.wails<ImportedContent>('ImportLesson', lessonId, { title, text, pdf_base64: '', image_base64: '' }).pipe(map((d) => ({ data: d })));
    return this.http.post<ApiResponse<ImportedContent>>(`${this.base}/lessons/${lessonId}/import`, { title, text });
  }
  importLessonPDF(lessonId: number, title: string, file: File): Observable<ApiResponse<ImportedContent>> {
    if (this.inWails()) {
      return from(file.arrayBuffer()).pipe(
        map((buf) => {
          const bytes = new Uint8Array(buf);
          let bin = '';
          for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
          const b64 = btoa(bin);
          return { title, text: '', pdf_base64: b64, image_base64: '' };
        }),
        (obs) => obs.pipe((req: any) => this.wails<ImportedContent>('ImportLesson', lessonId, req).pipe(map((d) => ({ data: d }))))
      );
    }
    const fd = new FormData();
    fd.append('file', file);
    if (title) fd.append('title', title);
    return this.http.post<ApiResponse<ImportedContent>>(`${this.base}/lessons/${lessonId}/import`, fd);
  }

  // ========== AI 实景对话 ==========

  createDialogueSession(lessonId: number, userRole: string): Observable<ApiResponse<DialogueSession>> {
    if (this.inWails()) return this.wails<DialogueSession>('CreateDialogueSession', lessonId, userRole).pipe(map((d) => ({ data: d })));
    return this.http.post<ApiResponse<DialogueSession>>(`${this.base}/dialogue/sessions`, { lesson_id: lessonId, user_role: userRole });
  }

  sendDialogueMessage(sessionId: number, text: string): Observable<ApiResponse<{ user: DialogueMessage; ai: DialogueMessage }>> {
    if (this.inWails()) return this.wails<{ user: DialogueMessage; ai: DialogueMessage }>('SendDialogueMessage', sessionId, text).pipe(map((d) => ({ data: d })));
    return this.http.post<ApiResponse<{ user: DialogueMessage; ai: DialogueMessage }>>(`${this.base}/dialogue/sessions/${sessionId}/messages`, { text });
  }

  /** 流式发送（SSE 或 Wails 事件）——返回 Observable 直接订阅消息块 */
  streamDialogue(sessionId: number, text: string, onComplete?: (ai: DialogueMessage) => void): Observable<string> {
    if (this.inWails()) {
      // Wails 事件通道：订阅 ai:delta / ai:done（事件回调在 Angular zone 外，必须包 ngZone.run 触发变更检测）
      return new Observable<string>((sub) => {
        const rt = (window as any).runtime;
        if (!rt?.EventsOn) { sub.error(new Error('runtime 不可用')); return; }
        const onDelta = (_: any, data: any) => {
          if (data?.session_id === sessionId) {
            this.ngZone.run(() => sub.next(data.delta || ''));
          }
        };
        const onDone = (_: any, data: any) => {
          if (data?.session_id !== sessionId) return;
          this.ngZone.run(() => {
            if (data.error) sub.error(new Error(data.error));
            else {
              if (data.message && onComplete) onComplete(data.message);
              sub.complete();
            }
          });
        };
        rt.EventsOn('ai:delta', onDelta);
        rt.EventsOn('ai:done', onDone);
        this.wails<void>('StartStreamDialogue', sessionId, text).subscribe({
          error: (e) => { sub.error(e); },
        });
        return () => {
          rt.EventsOff('ai:delta');
          rt.EventsOff('ai:done');
        };
      });
    }
    // HTTP SSE（现有）
    return new Observable<string>((sub) => {
      const ctrl = new AbortController();
      fetch(`${this.base}/dialogue/sessions/${sessionId}/messages/stream`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ text }),
        signal: ctrl.signal,
      }).then((resp) => {
        if (!resp.ok || !resp.body) { throw new Error('流式请求失败 ' + resp.status); }
        const reader = resp.body.getReader();
        const dec = new TextDecoder();
        const pump = () => reader.read().then(({ done, value }) => {
          if (done) { sub.complete(); return; }
          const chunk = dec.decode(value, { stream: true });
          for (const line of chunk.split('\n')) {
            if (!line.startsWith('data:')) continue;
            const payload = line.slice(5).trim();
            if (payload === '[DONE]') { sub.complete(); return; }
            try {
              const obj = JSON.parse(payload);
              if (obj.error) { sub.error(new Error(obj.error)); return; }
              if (obj.delta) sub.next(obj.delta);
              if (obj.event === 'done' && obj.ai && onComplete) onComplete(obj.ai);
            } catch { /* 忽略脏数据 */ }
          }
          pump();
        });
        pump().catch((e) => sub.error(e));
      }).catch((e) => sub.error(e));
      return () => ctrl.abort();
    });
  }

  // ========== 能力画像 ==========

  getAbilityProfile(): Observable<ApiResponse<AbilityProfile>> {
    if (this.inWails()) return this.wails<AbilityProfile>('GetAbilityProfile').pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<AbilityProfile>>(`${this.base}/ability-profile`);
  }

  // ========== 发音评测 ==========

  listPronunciations(): Observable<ApiResponse<PronunciationRecord[]>> {
    if (this.inWails()) return this.wails<PronunciationRecord[]>('ListPronunciations').pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<PronunciationRecord[]>>(`${this.base}/pronunciations`);
  }
  evaluatePronunciation(body: { title?: string; target_text: string; recognized_text: string; duration_ms: number; audio_base64: string }): Observable<ApiResponse<PronunciationRecord>> {
    if (this.inWails()) return this.wails<PronunciationRecord>('EvaluatePronunciation', body).pipe(map((d) => ({ data: d })));
    return this.http.post<ApiResponse<PronunciationRecord>>(`${this.base}/pronunciations`, body);
  }
  deletePronunciation(id: number): Observable<ApiResponse<{ ok: boolean }>> {
    if (this.inWails()) return this.wails<void>('DeletePronunciation', id).pipe(map(() => ({ data: { ok: true } })));
    return this.http.delete<ApiResponse<{ ok: boolean }>>(`${this.base}/pronunciations/${id}`);
  }

  // ========== 模型配置 ==========

  getAIConfig(): Observable<ApiResponse<AIConfig>> {
    if (this.inWails()) return this.wails<AIConfig>('GetAIConfig').pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<AIConfig>>(`${this.base}/config/ai`);
  }
  saveAIConfig(cfg: AIConfig): Observable<ApiResponse<{ ok: boolean; message: string }>> {
    if (this.inWails()) return this.wails<void>('SaveAIConfig', cfg).pipe(map(() => ({ data: { ok: true, message: '已保存' } })));
    return this.http.post<ApiResponse<{ ok: boolean; message: string }>>(`${this.base}/config/ai`, cfg);
  }
  /** 当前 TTS 生效通道状态 */
  ttsStatus(): Observable<ApiResponse<{ provider: string; model: string; edge_available: boolean; voice_label: string }>> {
    if (this.inWails()) return this.wails<{ provider: string; model: string; edge_available: boolean; voice_label: string }>('TTSStatus').pipe(map((d) => ({ data: d })));
    return this.http.get<ApiResponse<{ provider: string; model: string; edge_available: boolean; voice_label: string }>>(`${this.base}/tts/status`);
  }

  /** 测试连接（Wails 返回 (message, err)；HTTP 返回 {ok,type,message}） */
  testAIConfig(type: string, config: ProviderSetting): Observable<{ ok: boolean; type: string; message: string }> {
    if (this.inWails()) {
      return this.wails<string>('TestProvider', type, config).pipe(
        map((msg) => ({ ok: true, type, message: msg })),
      );
    }
    return this.http.post<{ ok: boolean; type: string; message: string }>(`${this.base}/config/ai/test`, { type, config });
  }
}
