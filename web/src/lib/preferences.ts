import { writable } from 'svelte/store';

export type Language = 'zh-CN' | 'en-US';
export type Theme = 'light' | 'dark';

export type Preferences = {
  language: Language;
  theme: Theme;
};

const defaultPreferences: Preferences = {
  language: 'zh-CN',
  theme: 'light'
};

export const preferences = writable<Preferences>(defaultPreferences);

export function initPreferences() {
  const next = readPreferences();
  preferences.set(next);
  applyPreferences(next);
}

export function toggleLanguage() {
  preferences.update((current) => {
    const next = {
      ...current,
      language: current.language === 'zh-CN' ? 'en-US' : 'zh-CN'
    };
    persistPreferences(next);
    applyPreferences(next);
    return next;
  });
}

export function toggleTheme() {
  preferences.update((current) => {
    const next = {
      ...current,
      theme: current.theme === 'light' ? 'dark' : 'light'
    };
    persistPreferences(next);
    applyPreferences(next);
    return next;
  });
}

function readPreferences(): Preferences {
  if (typeof localStorage === 'undefined') {
    return defaultPreferences;
  }
  return {
    language: localStorage.getItem('omo-language') === 'en-US' ? 'en-US' : 'zh-CN',
    theme: localStorage.getItem('omo-theme') === 'dark' ? 'dark' : 'light'
  };
}

function persistPreferences(next: Preferences) {
  if (typeof localStorage === 'undefined') {
    return;
  }
  localStorage.setItem('omo-language', next.language);
  localStorage.setItem('omo-theme', next.theme);
}

function applyPreferences(next: Preferences) {
  if (typeof document === 'undefined' || typeof window === 'undefined') {
    return;
  }
  document.documentElement.lang = next.language;
  document.documentElement.dataset.theme = next.theme;
  window.dispatchEvent(new CustomEvent<Language>('omo-language-change', { detail: next.language }));
}
