import { Injectable } from '@angular/core';

// ---- 与后端 JSON 对应的类型 ----
export interface Textbook {
  id: number;
  name: string;
  publisher: string;
  stage: string;
  grade: string;
  catalog_source: string;
  status: string;
}

export interface CatalogLesson {
  id: number;
  lesson: string;
  title: string;
  scene: string;
  status: string;
  has_content: boolean;
}

export interface CatalogNode {
  level: string;
  unit: string;
  title: string;
  lessons: CatalogLesson[];
}

export interface DialogueTurn {
  role: string;
  role_name: string;
  text: string;
  audio_url?: string;
  image?: string;
}

export interface VocabItem {
  word: string;
  phonetic: string;
  meaning: string;
  example: string;
  audio_url?: string;
}

export interface GrammarPoint {
  point: string;
  explanation: string;
  example: string;
}

export interface PracticeItem {
  type: string; // roleplay / speak / write
  prompt: string;
  example: string;
  hints: string[];
}

export interface LessonContent {
  id: number;
  catalog_item_id: number;
  unit: string;
  title: string;
  scene: string;
  dialogue: DialogueTurn[];
  key_vocab: VocabItem[];
  grammar_points: GrammarPoint[];
  practice: PracticeItem[];
  images: string[];
  svg_images: SVGImage[];
  role_voices: Record<string, string>;
  source: string;
  status: string;
}

/** 导入课文内容（真实课文原文，版权合规通道） */
export interface ImportedContent {
  id: number;
  catalog_item_id: number;
  unit: string;
  title: string;
  imported_text: string;
  source: string;
  status: string;
}

export interface AIRole {
  role: string;
  role_name: string;
}

export interface DialogueSession {
  id: number;
  lesson_id: number;
  scene: string;
  title: string;
  user_role: string;
  user_role_name: string;
  ai_roles: AIRole[];
  status: string;
}

export interface DialogueMessage {
  id: number;
  session_id: number;
  speaker: string;
  speaker_name: string;
  is_user: boolean;
  content: string;
  audio_url?: string;
  created_at: string;
}

export interface AbilityProfile {
  listening: number;
  speaking: number;
  reading: number;
  writing: number;
  total_activities: number;
  weakest: string;
  advice: string;
}

export interface SVGImage {
  title: string;
  svg: string;
}

export interface ProviderSetting {
  provider: string; // openai / mock / auto
  base_url: string;
  api_key: string;
  model: string;
  extra?: string;
}

export interface AIConfig {
  id: number;
  llm: ProviderSetting;
  tts: ProviderSetting;
  image: ProviderSetting;
  ocr: ProviderSetting;
  updated_at: string;
}

export interface TextbookPreset {
  name: string;
  publisher: string;
  stage: string;
  grade: string;
}

export interface TextbookMeta {
  textbooks: TextbookPreset[];
  stages: string[];
  grades: Record<string, string[]>;
}

/** 发音评测记录 */
export interface PronunciationRecord {
  id: number;
  title: string;
  target_text: string;
  recognized_text: string;
  accuracy: number;
  fluency: number;
  completeness: number;
  intonation: number;
  score: number;
  suggestion: string;
  audio_url: string;
  duration_ms: number;
  created_at: string;
}

export interface ApiResponse<T> {
  data: T;
  error?: string;
}
