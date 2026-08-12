import { Routes } from '@angular/router';
import { TextbooksComponent } from './pages/textbooks/textbooks.component';
import { DialogueComponent } from './pages/dialogue/dialogue.component';
import { AbilityComponent } from './pages/ability/ability.component';
import { ConfigComponent } from './pages/config/config.component';
import { PronunciationComponent } from './pages/pronunciation/pronunciation.component';

export const routes: Routes = [
  { path: '', redirectTo: 'textbooks', pathMatch: 'full' },
  { path: 'textbooks', component: TextbooksComponent },
  { path: 'dialogue/:lessonId', component: DialogueComponent },
  { path: 'ability', component: AbilityComponent },
  { path: 'config', component: ConfigComponent },
  { path: 'pronunciation', component: PronunciationComponent },
];
