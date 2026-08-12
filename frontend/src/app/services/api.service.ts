import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { Observable } from 'rxjs';
import { ApiResponse, AbilityProfile, AIConfig, CatalogNode, DialogueMessage, DialogueSession, ImportedContent, LessonContent, PronunciationRecord, ProviderSetting, Textbook, TextbookMeta } from '../models/models';

@Injectable({ providedIn: 'root' })
export class ApiService {
  // 统一相对路径：生产模式（Gin 托管）直接命中后端；
  // 开发模式（ng serve）由 proxy.conf.json 代理到 8090
  private base = '/api';

  /** 暴露 base 供流式 fetch 使用 */
  getBase(): string {
    return this.base;
  }

  constructor(private http: HttpClient) {}

  listTextbooks(): Observable<ApiResponse<Textbook[]>> {
    return this.http.get<ApiResponse<Textbook[]>>(`${this.base}/textbooks`);
  }

  createTextbook(body: { name: string; publisher?: string; stage?: string; grade?: string }): Observable<ApiResponse<Textbook>> {
    return this.http.post<ApiResponse<Textbook>>(`${this.base}/textbooks`, body);
  }

  getCatalog(textbookId: number): Observable<ApiResponse<CatalogNode[]>> {
    return this.http.get<ApiResponse<CatalogNode[]>>(`${this.base}/textbooks/${textbookId}/catalog`);
  }

  /** 异步拉取课时内容（后台任务，返回 task_id） */
  pullLessonContent(lessonId: number, force = false): Observable<{ task_id: string; status: string; message: string }> {
    return this.http.post<{ task_id: string; status: string; message: string }>(`${this.base}/lessons/${lessonId}/pull`, { force });
  }

  /** 异步任务状态（done 时含结果 content） */
  getTask(taskId: string): Observable<ApiResponse<{ id: string; status: string; error?: string; content?: LessonContent }>> {
    return this.http.get<ApiResponse<{ id: string; status: string; error?: string; content?: LessonContent }>>(`${this.base}/tasks/${taskId}`);
  }

  /** 导入课文（版权合规通道）：文本粘贴或 PDF 上传 */
  importLessonText(lessonId: number, title: string, text: string): Observable<ApiResponse<LessonContent>> {
    return this.http.post<ApiResponse<LessonContent>>(`${this.base}/lessons/${lessonId}/import`, { title, text });
  }
  importLessonPDF(lessonId: number, title: string, file: File): Observable<ApiResponse<LessonContent>> {
    const fd = new FormData();
    fd.append('file', file);
    if (title) fd.append('title', title);
    return this.http.post<ApiResponse<LessonContent>>(`${this.base}/lessons/${lessonId}/import`, fd);
  }

  /** 查询课时内容（kind=ai 默认 | imported，前端 Tab 切换） */
  getLessonContent(lessonId: number, kind = 'ai'): Observable<ApiResponse<LessonContent | ImportedContent>> {
    return this.http.get<ApiResponse<LessonContent | ImportedContent>>(`${this.base}/lessons/${lessonId}/content?kind=${kind}`);
  }

  /** 创建 AI 实景对话会话 */
  createDialogueSession(lessonId: number, userRole: string): Observable<ApiResponse<DialogueSession>> {
    return this.http.post<ApiResponse<DialogueSession>>(`${this.base}/dialogue/sessions`, { lesson_id: lessonId, user_role: userRole });
  }

  /** 发送对话消息 */
  sendDialogueMessage(sessionId: number, text: string): Observable<ApiResponse<{ user: DialogueMessage; ai: DialogueMessage }>> {
    return this.http.post<ApiResponse<{ user: DialogueMessage; ai: DialogueMessage }>>(
      `${this.base}/dialogue/sessions/${sessionId}/messages`, { text }
    );
  }

  /** 能力画像（雷达图） */
  getAbilityProfile(): Observable<ApiResponse<AbilityProfile>> {
    return this.http.get<ApiResponse<AbilityProfile>>(`${this.base}/ability-profile`);
  }

  /** 删除教材（连带目录与内容） */
  deleteTextbook(id: number): Observable<ApiResponse<{ ok: boolean; message: string }>> {
    return this.http.delete<ApiResponse<{ ok: boolean; message: string }>>(`${this.base}/textbooks/${id}`);
  }

  /** 调整目录顺序（拖拽保存） */
  reorderCatalog(textbookId: number, ids: number[]): Observable<ApiResponse<{ ok: boolean; message: string }>> {
    return this.http.put<ApiResponse<{ ok: boolean; message: string }>>(`${this.base}/textbooks/${textbookId}/catalog/order`, { ids });
  }

  /** 调整教材库顺序（箭头排序保存） */
  reorderTextbooks(ids: number[]): Observable<ApiResponse<{ ok: boolean; message: string }>> {
    return this.http.put<ApiResponse<{ ok: boolean; message: string }>>(`${this.base}/textbooks/order`, { ids });
  }

  /** 常用教材预设（下拉列表） */
  getTextbookMeta(): Observable<ApiResponse<TextbookMeta>> {
    return this.http.get<ApiResponse<TextbookMeta>>(`${this.base}/textbooks/meta`);
  }

  /** 发音评测 */
  listPronunciations(): Observable<ApiResponse<PronunciationRecord[]>> {
    return this.http.get<ApiResponse<PronunciationRecord[]>>(`${this.base}/pronunciations`);
  }
  evaluatePronunciation(body: { title?: string; target_text: string; recognized_text: string; duration_ms: number; audio_base64: string }): Observable<ApiResponse<PronunciationRecord>> {
    return this.http.post<ApiResponse<PronunciationRecord>>(`${this.base}/pronunciations`, body);
  }
  deletePronunciation(id: number): Observable<ApiResponse<{ ok: boolean }>> {
    return this.http.delete<ApiResponse<{ ok: boolean }>>(`${this.base}/pronunciations/${id}`);
  }

  /** 模型配置 */
  getAIConfig(): Observable<ApiResponse<AIConfig>> {
    return this.http.get<ApiResponse<AIConfig>>(`${this.base}/config/ai`);
  }
  saveAIConfig(cfg: AIConfig): Observable<ApiResponse<{ ok: boolean; message: string }>> {
    return this.http.post<ApiResponse<{ ok: boolean; message: string }>>(`${this.base}/config/ai`, cfg);
  }
  /** 测试连接（返回体无 data 包装：{ok, type, message}） */
  testAIConfig(type: string, config: ProviderSetting): Observable<{ ok: boolean; type: string; message: string }> {
    return this.http.post<{ ok: boolean; type: string; message: string }>(`${this.base}/config/ai/test`, { type, config });
  }
}
