import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterLink } from '@angular/router';
import { ApiService } from '../../services/api.service';

// Plotly 类型（plotly.js-dist-min 无自带类型，最小声明）
declare const Plotly: any;

export interface AbilityProfile {
  listening: number;
  speaking: number;
  reading: number;
  writing: number;
  total_activities: number;
  weakest: string;
  advice: string;
}

@Component({
  selector: 'app-ability',
  standalone: true,
  imports: [CommonModule, RouterLink],
  templateUrl: './ability.component.html',
  styleUrls: ['./ability.component.css'],
})
export class AbilityComponent implements OnInit {
  profile: AbilityProfile | null = null;
  error = '';

  constructor(private api: ApiService) {}

  ngOnInit(): void {
    this.api.getAbilityProfile().subscribe({
      next: (res) => {
        this.profile = res.data;
        this.renderRadar(res.data);
      },
      error: (e) => (this.error = '加载能力画像失败：' + (e.error?.error || e.message)),
    });
  }

  /** Plotly 四维雷达图 */
  private renderRadar(p: AbilityProfile): void {
    const values = [p.listening, p.speaking, p.reading, p.writing];
    // 雷达图需首尾闭合
    const labels = ['听力', '口语', '阅读', '写作'];
    const closedValues = [...values, values[0]];
    const closedLabels = [...labels, labels[0]];

    const data = [{
      type: 'scatterpolar',
      r: closedValues,
      theta: closedLabels,
      fill: 'toself',
      line: { color: '#1a73e8', width: 2 },
      fillcolor: 'rgba(26,115,232,0.25)',
      name: '能力值',
    }];
    const layout = {
      polar: {
        radialaxis: { visible: true, range: [0, 100], tickfont: { size: 11 } },
      },
      margin: { l: 40, r: 40, t: 10, b: 30 },
      showlegend: false,
      paper_bgcolor: 'rgba(0,0,0,0)',
      font: { family: 'Segoe UI, Microsoft YaHei, sans-serif' },
    };
    Plotly.newPlot('ability-radar', data, layout, { responsive: true, displayModeBar: false });
  }
}
