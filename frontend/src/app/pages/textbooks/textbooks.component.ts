import { Component, DestroyRef, inject, OnDestroy, OnInit } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { filter, interval, Subscription, switchMap, take } from 'rxjs';
import { ApiService } from '../../services/api.service';
import { AudioService } from '../../services/audio.service';
import { ImportedContent, TextbookMeta } from '../../models/models';
import {
  CatalogLesson,
  CatalogNode,
  LessonContent,
  Textbook,
} from '../../models/models';

@Component({
  selector: 'app-textbooks',
  standalone: true,
  imports: [CommonModule, FormsModule, RouterLink],
  templateUrl: './textbooks.component.html',
  styleUrls: ['./textbooks.component.css'],
})
export class TextbooksComponent implements OnInit, OnDestroy {
  // 创建表单
  form = { name: '', publisher: '', stage: '初中', grade: '七年级' };
  creating = false;

  // 教材预设下拉
  meta: TextbookMeta | null = null;
  customName = false; // 是否选择"自定义教材"

  // sidebar 收缩（为单元/课文内容留出空间）
  sidebarCollapsed = false;

  toggleSidebar(): void {
    this.sidebarCollapsed = !this.sidebarCollapsed;
  }

  // 教材库
  textbooks: Textbook[] = [];
  selectedTextbook: Textbook | null = null;
  catalog: CatalogNode[] = [];
  loadingCatalog = false;

  // 课时
  selectedLesson: CatalogLesson | null = null;
  activeTab: 'ai' | 'imported' = 'ai'; // 🎨 AI 创作 / 📖 真实课文
  content: LessonContent | null = null; // AI 创作内容
  importedContent: ImportedContent | null = null; // 导入课文内容
  loadingContent = false;
  error = '';
  /** 全文朗读状态（readingIdx=当前播放句下标，-1 未播放） */
  readingAll = false;
  readingIdx = -1;

  // 角色头像 emoji
  private roleIcons: Record<string, string> = {
    waiter: '🧑‍🍳', customer: '🙋', clerk: '🧑‍💼', teacher: '👩‍🏫',
    student: '🧒', doctor: '🧑‍⚕️', driver: '🧑‍✈️', friend: '🧑‍🤝‍🧑',
  };
  private roleColors: Record<string, string> = {
    waiter: '#1a73e8', customer: '#188038', clerk: '#7b1fa2',
    teacher: '#e37400', student: '#d93025', doctor: '#00838f',
    driver: '#5f6368', friend: '#b06000',
  };

  constructor(private api: ApiService, private audio: AudioService) {
    this.destroyRef = inject(DestroyRef);
  }

  ngOnDestroy(): void {
    this.audio.stop(); // 离开页面中止朗读
    this.cancelPolling();
  }

  private destroyRef!: DestroyRef;

  ngOnInit(): void {
    this.loadTextbooks();
    this.loadMeta();
  }

  /** 加载常用教材预设（下拉数据源） */
  loadMeta(): void {
    this.api.getTextbookMeta().subscribe({
      next: (res) => (this.meta = res.data),
      error: () => (this.meta = null),
    });
  }

  /** 当前阶段可选的教材 */
  get filteredTextbooks() {
    if (!this.meta) return [];
    return this.meta.textbooks.filter((t) => t.stage === this.form.stage);
  }

  /** 教材库列表（跟随创建表单的阶段过滤） */
  get libraryTextbooks(): Textbook[] {
    if (!this.form.stage) return this.textbooks;
    return this.textbooks.filter((t) => t.stage === this.form.stage);
  }

  /** 当前阶段可选年级 */
  get gradeOptions(): string[] {
    if (!this.meta) return [];
    return this.meta.grades[this.form.stage] || [];
  }

  /** 切换阶段：重置教材/年级为默认 */
  onStageChange(): void {
    this.form.name = '';
    this.form.grade = this.gradeOptions[0] || '';
    this.customName = false;
  }

  /** 选择教材预设：自动带出出版社与年级；自定义则手动填写 */
  onTextbookSelect(): void {
    if (this.form.name === '__custom__') {
      this.customName = true;
      this.form.name = '';
      this.form.publisher = '';
      return;
    }
    this.customName = false;
    if (!this.meta || !this.form.name) return;
    const preset = this.meta.textbooks.find((t) => t.name === this.form.name);
    if (preset) {
      this.form.publisher = preset.publisher;
      this.form.grade = preset.grade;
    }
  }

  /** 统一错误信息提取 */
  private errMsg(e: any): string {
    return e?.error?.error || e?.message || '请求失败';
  }

  loadTextbooks(): void {
    this.api.listTextbooks().subscribe({
      next: (res) => {
        this.textbooks = res.data;
        if (this.textbooks.length && !this.selectedTextbook) {
          this.selectTextbook(this.textbooks[0]);
        }
      },
      error: (e) => (this.error = '加载教材失败：' + this.errMsg(e)),
    });
  }

  /** 教材库排序（箭头按钮，跨阶段整体排序） */
  moveTextbook(i: number, dir: number): void {
    const fl = this.libraryTextbooks;
    const cur = fl[i], nxt = fl[i + dir];
    if (!cur || !nxt) return;
    const ci = this.textbooks.indexOf(cur), ni = this.textbooks.indexOf(nxt);
    [this.textbooks[ci], this.textbooks[ni]] = [this.textbooks[ni], this.textbooks[ci]];
    this.saveTextbookOrder();
  }

  saveTextbookOrder(): void {
    const ids = this.textbooks.map((t) => t.id);
    this.api.reorderTextbooks(ids).subscribe({
      next: (res) => { this.notice = '✅ ' + res.data.message; },
      error: (e) => { this.error = '保存教材顺序失败：' + this.errMsg(e); this.loadTextbooks(); },
    });
  }

  /** 删除教材（连带目录与内容），确认后执行 */
  removeTextbook(tb: Textbook, e: Event): void {
    e.stopPropagation(); // 不触发行选择
    if (!confirm(`确定删除「${tb.name}」？\n其目录、课文内容、对话记录将一并删除，不可恢复。`)) return;
    this.api.deleteTextbook(tb.id).subscribe({
      next: () => {
        this.error = '';
        this.notice = `🗑 已删除教材「${tb.name}」`;
        if (this.selectedTextbook?.id === tb.id) {
          this.selectedTextbook = null;
          this.selectedLesson = null;
          this.content = null;
          this.importedContent = null;
        }
        this.loadTextbooks();
      },
      error: (e) => (this.error = '删除失败：' + this.errMsg(e)),
    });
  }

  createTextbook(): void {
    if (!this.form.name.trim()) return;
    this.creating = true;
    this.error = '';
    this.api.createTextbook(this.form).subscribe({
      next: () => {
        this.creating = false;
        this.form.name = '';
        this.loadTextbooks();
      },
      error: (e) => {
        this.creating = false;
        this.error = '创建失败：' + this.errMsg(e);
      },
    });
  }

  selectTextbook(tb: Textbook): void {
    this.selectedTextbook = tb;
    this.selectedLesson = null;
    this.content = null;
    this.importedContent = null;
    this.loadCatalog(tb.id);
  }

  loadCatalog(textbookId: number): void {
    this.loadingCatalog = true;
    this.api.getCatalog(textbookId).subscribe({
      next: (res) => {
        this.catalog = res.data;
        this.loadingCatalog = false;
      },
      error: (e) => {
        this.loadingCatalog = false;
        this.error = '加载目录失败：' + this.errMsg(e);
      },
    });
  }

  selectLesson(lesson: CatalogLesson): void {
    this.selectedLesson = lesson;
    this.content = null;
    this.importedContent = null;
    this.activeTab = 'ai';
    this.audio.stop(); // 切换课时中止朗读
    this.readingAll = false;
    this.readingIdx = -1;
    this.error = '';
    this.notice = '';
    this.cancelPolling(); // 取消当前轮询订阅（不删除进行中任务记录）
    // 该课时有进行中任务 → 恢复"拉取中"状态并继续跟随
    const taskId = this.polling.get(lesson.id);
    if (taskId) {
      this.loadingContent = true;
      this.notice = '⏳ AI 生成中…（后台继续，完成后自动显示）';
      this.pollTask(taskId, lesson);
    } else if (lesson.has_content) {
      this.loadContent(lesson.id, 'ai');
    }
  }

  /** 保存目录顺序（拖拽后自动调用） */
  saveOrder(): void {
    if (!this.selectedTextbook) return;
    const ids = this.catalog.flatMap((n) => n.lessons.map((l) => l.id));
    this.api.reorderCatalog(this.selectedTextbook.id, ids).subscribe({
      next: (res) => { this.notice = '✅ ' + res.data.message; },
      error: (e) => {
        this.error = '保存顺序失败：' + this.errMsg(e);
        if (this.selectedTextbook) this.loadCatalog(this.selectedTextbook.id);
      },
    });
  }

  // ---- 排序（箭头按钮，可靠）----
  moveUnit(dir: number, i: number): void {
    const j = i + dir;
    if (j < 0 || j >= this.catalog.length) return;
    [this.catalog[i], this.catalog[j]] = [this.catalog[j], this.catalog[i]];
    this.saveOrder();
  }

  moveLesson(ui: number, li: number, dir: number): void {
    const lessons = this.catalog[ui].lessons;
    const j = li + dir;
    if (j < 0 || j >= lessons.length) return;
    [lessons[li], lessons[j]] = [lessons[j], lessons[li]];
    this.saveOrder();
  }

  /** 切换 Tab：AI 创作 / 真实课文（按需加载对应 kind 内容） */
  switchTab(tab: 'ai' | 'imported'): void {
    if (!this.selectedLesson || this.activeTab === tab) return;
    this.activeTab = tab;
    this.error = '';
    this.notice = '';
    this.audio.stop(); // 切 Tab 中止朗读
    this.readingAll = false;
    this.readingIdx = -1;
    this.loadContent(this.selectedLesson.id, tab);
  }

  loadContent(lessonId: number, kind: 'ai' | 'imported'): void {
    this.loadingContent = true;
    this.api.getLessonContent(lessonId, kind).subscribe({
      next: (res) => {
        if (kind === 'imported') this.importedContent = res.data as ImportedContent;
        else this.content = res.data as LessonContent;
        this.loadingContent = false;
      },
      error: (e) => {
        this.loadingContent = false;
        // 未生成内容不报错（显示对应空态；AI 与导入课文独立，用户手动点 Tab 切换）
        if (kind === 'imported') this.importedContent = null;
        else this.content = null;
        this.error = '';
      },
    });
  }

  /** 图片 URL 转换（Wails 下转 file://，浏览器下原样） */
  imgSrc(img: string): string {
    return this.api.mediaUrl(img);
  }

  /** 从大模型拉取该课时内容（异步后台任务，切页面不中断） */
  pullContent(lesson: CatalogLesson): void {
    this.loadingContent = true;
    this.error = '';
    this.notice = '';
    this.api.pullLessonContent(lesson.id).subscribe({
      next: (res) => {
        this.polling.set(lesson.id, res.task_id); // 记录进行中任务（切课时保留）
        this.notice = '⏳ AI 生成中…可切到其他页面，完成后自动保存（稍后回来即可见）';
        this.pollTask(res.task_id, lesson);
      },
      error: (e) => {
        this.loadingContent = false;
        this.error = '启动拉取失败：' + this.errMsg(e);
      },
    });
  }

  /** 修复历史外链图片：批量下载到本地（一次性，图片永久化） */
  fixImages(): void {
    if (!confirm('把历史课文里的外链图片（硅基流动临时 URL / unsplash）批量下载到本地，永久保存？\n（下载失败的保留原外链）')) return;
    this.error = '';
    this.notice = '⏳ 正在下载外链图片…请稍候';
    this.api.migrateImages().subscribe({
      next: (res) => {
        const d = res.data;
        this.notice = `✅ 修复完成：${d.downloaded} 张图片已下载到本地（涉及 ${d.lessons} 个课时）` + (d.failed ? `，${d.failed} 张下载失败（保留原外链）` : '');
        if (this.selectedTextbook) this.loadCatalog(this.selectedTextbook.id); // 刷新目录
      },
      error: (e) => {
        this.error = '图片修复失败：' + this.errMsg(e);
        this.notice = '';
      },
    });
  }

  /** 重新生成：强制覆盖当前课文内容（异步） */
  regenerate(): void {
    if (!this.selectedLesson) return;
    if (!confirm('重新生成将覆盖当前课文内容（图片/对话/练习都会重新生成），确定继续？')) return;
    this.loadingContent = true;
    this.error = '';
    this.notice = '';
    this.api.pullLessonContent(this.selectedLesson.id, true).subscribe({
      next: (res) => {
        this.polling.set(this.selectedLesson!.id, res.task_id);
        this.notice = '⏳ 重新生成中…完成后自动保存';
        this.pollTask(res.task_id, this.selectedLesson!);
      },
      error: (e) => {
        this.loadingContent = false;
        this.error = '启动重新生成失败：' + this.errMsg(e);
      },
    });
  }

  /** 轮询异步任务（每 2s；切课时取消、组件销毁停止——但后端任务继续） */
  private taskPoll: Subscription | null = null;
  private polling = new Map<number, string>(); // lessonId → taskId（进行中任务记录，切课时保留）

  private pollTask(taskId: string, lesson: CatalogLesson): void {
    this.taskPoll?.unsubscribe();
    this.taskPoll = interval(2000).pipe(
      takeUntilDestroyed(this.destroyRef),
      switchMap(() => this.api.getTask(taskId)),
      filter((res) => res.data.status !== 'running'),
      take(1)
    ).subscribe({
      next: (res) => {
        this.taskPoll = null;
        this.loadingContent = false;
        this.polling.delete(lesson.id); // 任务结束，移除进行中记录
        // 已切到其他课时：忽略结果（内容已入库，切回来自动可见）
        if (this.selectedLesson?.id !== lesson.id) return;
        const t = res.data;
        if (t.status === 'done' && t.content) {
          this.content = t.content;
          lesson.has_content = true;
          lesson.status = 'published';
          this.notice = '✅ AI 内容已生成并保存（点击「🔄 重新生成」可换一批）';
        } else if (t.status === 'error') {
          this.error = '拉取失败：' + (t.error || '未知错误');
        }
      },
      error: () => {
        this.taskPoll = null;
        this.loadingContent = false;
        this.polling.delete(lesson.id);
        this.error = '任务查询失败，请稍后刷新页面查看内容';
      },
    });
  }

  /** 取消当前轮询订阅（切换课时时调用；进行中任务记录保留，切回可恢复） */
  private cancelPolling(): void {
    this.taskPoll?.unsubscribe();
    this.taskPoll = null;
    this.loadingContent = false;
  }

  // ---- 主内容区提示（成功/失败，替代侧栏不可见的错误）----
  notice = '';

  // ---- 课文导入（版权合规通道）----
  showImport = false;
  importTitle = '';
  importText = '';
  importFile: File | null = null;
  importing = false;

  toggleImport(): void {
    this.showImport = !this.showImport;
  }

  onFileSelected(e: Event): void {
    const el = e.target as HTMLInputElement;
    this.importFile = el.files?.[0] || null;
  }

  /** 提交导入（PDF 或文本） */
  submitImport(): void {
    if (!this.selectedLesson || this.importing) return;
    if (!this.importFile && !this.importText.trim()) {
      this.error = '请粘贴课文文本或选择 PDF 文件';
      return;
    }
    this.importing = true;
    this.error = '';
    const done = () => { this.importing = false; this.showImport = false; this.importFile = null; this.importText = ''; };
    const use = (obs: any) => obs.subscribe({
      next: (res: any) => {
        this.importedContent = res.data;
        this.selectedLesson!.has_content = true;
        this.activeTab = 'imported'; // 导入后自动切到「真实课文」Tab
        done();
      },
      error: (e: any) => { this.importing = false; this.error = '导入失败：' + this.errMsg(e); },
    });
    if (this.importFile) {
      const name = this.importFile.name.toLowerCase();
      const isImg = name.endsWith('.png') || name.endsWith('.jpg') || name.endsWith('.jpeg') || name.endsWith('.webp');
      if (isImg) {
        use(this.api.importLessonImage(this.selectedLesson.id, this.importTitle, this.importFile));
      } else {
        use(this.api.importLessonPDF(this.selectedLesson.id, this.importTitle, this.importFile));
      }
    } else {
      use(this.api.importLessonText(this.selectedLesson.id, this.importTitle, this.importText));
    }
  }

  /** 角色音色中文标签（多角色对话气泡显示） */
  roleVoice(role: string): string {
    const m: Record<string, string> = { boy: '男童音色', girl: '女童音色', student: '童声音色', teacher: '年轻女教师音色', male_teacher: '年轻男教师音色', grandpa: '老年男声音色', grandma: '老年女声音色', narrator: '旁白', brother: '青年男声', sister: '青年女声', uncle: '中年男声', aunt: '中年女声', mother: '妈妈音色', father: '爸爸音色', waiter: '男声', customer: '女声', farmer: '浑厚男声', worker: '男声', doctor: '女声', nurse: '女声', police: '男声', friend: '女声' };
    return m[role] || '标准发音';
  }

  /** 导入课文多角色对话全文朗读（逐句按角色音色轮播） */
  playImportedDialogue(): void {
    const items = (this.importedContent?.dialogue ?? []).map((d) => ({ text: d.text, url: d.audio_url }));
    if (!items.length) return;
    this.readingAll = true;
    this.readingIdx = -1;
    this.audio.playQueue(items,
      (i) => { this.readingIdx = i; },
      () => { this.readingAll = false; this.readingIdx = -1; }
    );
  }

  /** 朗读文本：优先 TTS URL，无则浏览器朗读 */
  play(text: string, url?: string): void {
    this.audio.speak(text, url);
  }

  /** 全文朗读：按对话顺序逐句播放，每句自动用对应角色的音色（teacher/boy/girl 不同声音） */
  playAll(): void {
    const items = (this.content?.dialogue ?? []).map((d) => ({ text: d.text, url: d.audio_url }));
    if (!items.length) return;
    this.readingAll = true;
    this.readingIdx = -1;
    this.audio.playQueue(items,
      (i) => { this.readingIdx = i; },   // 高亮当前句
      () => { this.readingAll = false; this.readingIdx = -1; } // 播完复位
    );
  }

  /** 停止全文朗读 */
  stopAll(): void {
    this.audio.stop();
    this.readingAll = false;
    this.readingIdx = -1;
  }

  /** 对话角色是否足够（≥2 才可进行情景对话） */
  canDialogue(): boolean {
    return !!this.content && this.content.dialogue.length >= 2;
  }

  /** 是否有 SVG 示意图 */
  hasSVGs(): boolean {
    return !!this.content && this.content.svg_images?.length > 0;
  }

  /** SVG 源码 → data URL（<img> 渲染天然不执行脚本，安全） */
  svgDataUrl(svg: string): string {
    const bytes = new TextEncoder().encode(svg);
    let bin = '';
    for (const b of bytes) bin += String.fromCharCode(b);
    return 'data:image/svg+xml;base64,' + btoa(bin);
  }

  stopPlay(): void {
    this.audio.stop();
  }

  roleIcon(role: string): string {
    return this.roleIcons[role] ?? '💬';
  }

  roleColor(role: string): string {
    return this.roleColors[role] ?? '#5f6368';
  }

  practiceLabel(type: string): string {
    return { roleplay: '🎭 情景演练', speak: '🗣️ 开口说', write: '✍️ 写一写' }[type] ?? type;
  }
}
