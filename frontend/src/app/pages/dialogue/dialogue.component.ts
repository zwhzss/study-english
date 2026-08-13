import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { ApiService } from '../../services/api.service';
import { LessonContent } from '../../models/models';
import { AudioService } from '../../services/audio.service';
import { SpeechService } from '../../services/speech.service';
import { timeout } from 'rxjs';

interface RoleOption {
  role: string;
  role_name: string;
  icon: string;
}

interface ChatMsg {
  speaker: string;
  speaker_name: string;
  is_user: boolean;
  content: string;
  audio_url?: string;
  streaming?: boolean; // 流式输出中（显示光标）
}

const ROLE_ICONS: Record<string, string> = {
  waiter: '🧑‍🍳', customer: '🙋', clerk: '🧑‍💼', teacher: '👩‍🏫',
  student: '🧒', doctor: '🧑‍⚕️', driver: '🧑‍✈️', friend: '🧑‍🤝‍🧑',
};

@Component({
  selector: 'app-dialogue',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './dialogue.component.html',
  styleUrls: ['./dialogue.component.css'],
})
export class DialogueComponent implements OnInit, OnDestroy {
  lessonId = 0;
  freeTalk = false;
  content: LessonContent | null = null;
  roles: RoleOption[] = [];
  sessionId = 0;
  sessionTitle = '';
  sessionScene = '';

  // 对话状态
  messages: ChatMsg[] = [];
  inputText = '';
  sending = false;
  interimText = ''; // 语音识别中间结果
  recogBusy = false; // 录音识别中
  micOn = false;
  micSupported = true;
  error = '';
  private streamAbort: AbortController | null = null;
  private sseErrored = false;

  constructor(
    private route: ActivatedRoute,
    private router: Router,
    private api: ApiService,
    private audio: AudioService,
    private speech: SpeechService
  ) {}

  ngOnInit(): void {
    this.micSupported = true; // MediaRecorder 由运行时检测
    this.route.paramMap.subscribe((p) => {
      const id = p.get('lessonId') || '0';
      if (id === 'free') {
        this.freeTalk = true;
        this.lessonId = 0;
        this.roles = [
          { role: 'student', role_name: '学生', icon: ROLE_ICONS['student'] },
          { role: 'teacher', role_name: '英语老师', icon: ROLE_ICONS['teacher'] },
          { role: 'friend', role_name: '外国朋友', icon: ROLE_ICONS['friend'] },
        ];
        this.sessionTitle = '自由对话';
        this.sessionScene = 'Free Talk';
      } else {
        this.lessonId = Number(id) || 0;
        this.loadContent(this.lessonId);
      }
    });
  }

  ngOnDestroy(): void {
    this.speech.stop();
    this.streamAbort?.abort(); // 取消进行中的流式请求
  }

  loadContent(lessonId: number): void {
    this.api.getLessonContent(lessonId, 'ai').subscribe({
      next: (res) => {
        this.content = res.data as LessonContent;
        if (!this.content) return;
        this.sessionTitle = this.content.title;
        this.sessionScene = this.content.scene;
        // 提取可扮演角色（按出现顺序去重）
        const seen = new Set<string>();
        for (const t of this.content.dialogue) {
          if (!seen.has(t.role)) {
            seen.add(t.role);
            this.roles.push({ role: t.role, role_name: t.role_name, icon: ROLE_ICONS[t.role] || '💬' });
          }
        }
        if (this.roles.length < 2) this.error = '该课时对话角色不足，无法进行情景对话。';
      },
      error: (e) => {
        this.error = '加载课时失败：' + (e.error?.error || e.message);
      },
    });
  }

  iconOf(role: string): string {
    return ROLE_ICONS[role] || '💬';
  }

  /** 选择扮演角色 → 创建会话 */
  selectRole(role: string): void {
    this.error = '';
    this.api.createDialogueSession(this.lessonId, role).subscribe({
      next: (res) => {
        this.sessionId = res.data.id;
        this.sessionTitle = res.data.title;
        this.sessionScene = res.data.scene;
        this.messages.push({ speaker: '', speaker_name: '我', is_user: true, content: '' });
        this.messages.push({
          speaker: 'system', speaker_name: '系统', is_user: false,
          content: `情景已就绪：你扮演${res.data.user_role_name}，与 ${res.data.ai_roles.map((r: any) => r.role_name).join('、')} 对话。点击麦克风说话或输入英文开始！`,
        });
        this.scrollToBottom();
      },
      error: (e) => (this.error = '创建会话失败：' + (e.error?.error || e.message)),
    });
  }

  /** 发送文本消息（SSE 流式：AI 回复逐字显示） */
  send(text?: string): void {
    const content = (text ?? this.inputText).trim();
    if (!content || !this.sessionId || this.sending) return;
    this.inputText = '';
    this.sending = true;
    this.error = '';
    this.sseErrored = false;
    this.streamAbort = new AbortController();

    // 本地先添加用户消息 + AI 气泡（流式填充）
    this.messages.push({ speaker: '', speaker_name: '我', is_user: true, content });
    const aiBubble: ChatMsg = { speaker: 'ai', speaker_name: 'AI', is_user: false, content: '', streaming: true };
    this.messages.push(aiBubble);
    this.scrollToBottom();

    this.streamChat(this.sessionId, content, aiBubble, this.streamAbort.signal).catch((e) => {
      this.error = '发送失败：' + (e?.message || '请检查后端是否运行');
    }).finally(() => {
      this.sending = false;
    });
  }

  /** 流式聊天：双通道（Wails 事件 / HTTP SSE），逐字填充 AI 气泡 */
  private async streamChat(sessionId: number, text: string, aiBubble: ChatMsg, signal: AbortSignal): Promise<void> {
    await new Promise<void>((resolve, reject) => {
      const sub = this.api.streamDialogue(sessionId, text, (ai) => {
        // 完成：填充 AI 消息字段 + 自动朗读
        aiBubble.content = ai.content;
        aiBubble.speaker = ai.speaker;
        aiBubble.speaker_name = ai.speaker_name;
        aiBubble.audio_url = ai.audio_url;
        this.audio.speak(ai.content, ai.audio_url);
        this.scrollToBottom();
      }).pipe(timeout(30000)).subscribe({
        next: (delta) => {
          aiBubble.content += delta;
          this.scrollToBottom();
        },
        complete: () => {
          aiBubble.streaming = false;
          resolve();
        },
        error: (e) => {
          aiBubble.streaming = false;
          reject(e);
        },
      });
      signal.addEventListener('abort', () => sub.unsubscribe());
    });
  }

  /** 麦克风：录音 → 后端 ASR 识别 → 自动发送（原声不上传 AI，仅本地转文字） */
  toggleMic(): void {
    if (this.micOn) {
      // 点停止：停止录音并识别发送；按钮恢复麦克风
      this.speech.stop();
      this.micOn = false;
      this.interimText = '';
      return;
    }
    // 点麦克风：开始录音（按钮变红色停止）
    this.micOn = true;
    this.interimText = '';
    this.recogBusy = true;
    this.speech.start({
      onResult: (text) => {
        this.recogBusy = false;
        this.micOn = false;
        this.inputText = text;
        this.interimText = '';
        this.send(text); // 识别到即自动发送
      },
      onError: (msg) => {
        this.recogBusy = false;
        this.micOn = false;
        this.error = msg;
      },
      onState: (r) => {
        this.micOn = r;
      },
    });
  }

  /** 朗读某条消息 */
  playMsg(m: ChatMsg): void {
    this.audio.speak(m.content, m.audio_url);
  }

  backToLesson(): void {
    if (this.freeTalk || this.lessonId === 0) this.router.navigate(['/textbooks']);
    else this.router.navigate(['/textbooks']);
  }

  private scrollToBottom(): void {
    const el = document.querySelector('.msg-list');
    if (el) el.scrollTop = el.scrollHeight;
  }
}
