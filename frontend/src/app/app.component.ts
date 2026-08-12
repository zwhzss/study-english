import { Component, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterOutlet } from '@angular/router';

/**
 * 应用根组件：
 * - 页面缩放（Wails/WebView2 禁用了浏览器 Ctrl+± 缩放，前端自带缩放控件 + Ctrl+滚轮）
 * - 缩放级别用 CSS zoom（Chromium 原生支持，全局布局等比缩放），localStorage 记忆
 */
@Component({
  selector: 'app-root',
  imports: [RouterOutlet, CommonModule],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css'
})
export class AppComponent {
  title = 'AI 英语课堂';
  zoom = 1;
  private readonly ZOOM_MIN = 0.8;
  private readonly ZOOM_MAX = 1.5;
  private readonly ZOOM_STEP = 0.1;

  ngOnInit(): void {
    const saved = localStorage.getItem('app-zoom');
    if (saved) this.applyZoom(parseFloat(saved) || 1);
  }

  zoomIn(): void { this.setZoom(this.zoom + this.ZOOM_STEP); }
  zoomOut(): void { this.setZoom(this.zoom - this.ZOOM_STEP); }
  resetZoom(): void { this.setZoom(1); }

  setZoom(z: number): void {
    this.applyZoom(Math.min(this.ZOOM_MAX, Math.max(this.ZOOM_MIN, z)));
  }

  private applyZoom(z: number): void {
    this.zoom = Math.round(z * 100) / 100;
    document.body.style.zoom = String(this.zoom);
    try { localStorage.setItem('app-zoom', String(this.zoom)); } catch { /* ignore */ }
  }

  /** Ctrl+滚轮 缩放 */
  @HostListener('window:wheel', ['$event'])
  onWheel(e: WheelEvent): void {
    if (!e.ctrlKey) return;
    e.preventDefault();
    this.setZoom(this.zoom + (e.deltaY < 0 ? this.ZOOM_STEP : -this.ZOOM_STEP));
  }

  /** Ctrl+± 缩放（补齐被 Wails 禁用的浏览器快捷键） */
  @HostListener('window:keydown', ['$event'])
  onKey(e: KeyboardEvent): void {
    if (!e.ctrlKey) return;
    if (e.key === '=' || e.key === '+' || e.key === 'Add') {
      e.preventDefault();
      this.zoomIn();
    } else if (e.key === '-' || e.key === 'Subtract') {
      e.preventDefault();
      this.zoomOut();
    } else if (e.key === '0') {
      e.preventDefault();
      this.resetZoom();
    }
  }
}
