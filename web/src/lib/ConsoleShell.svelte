<script lang="ts">
  import Languages from '@lucide/svelte/icons/languages';
  import Moon from '@lucide/svelte/icons/moon';
  import Sun from '@lucide/svelte/icons/sun';
  import type { Snippet } from 'svelte';
  import { page } from '$app/stores';
  import { consoleLabels, consoleNavItems } from '$lib/consoleCopy';
  import { preferences, toggleLanguage, toggleTheme } from '$lib/preferences';

  let {
    title,
    eyebrow,
    activeHref = '',
    statusLabel = '',
    statusReady = false,
    actions,
    children
  }: {
    title: string;
    eyebrow?: string;
    activeHref?: string;
    statusLabel?: string;
    statusReady?: boolean;
    actions?: Snippet;
    children: Snippet;
  } = $props();

  let labels = $derived(consoleLabels($preferences.language));

  function isActive(href: string) {
    const path = activeHref || $page.url.pathname;
    return path === href || (href === '/services' && path === '/');
  }
</script>

<div class="shell">
  <aside class="sidebar" aria-label={labels.operationsConsole}>
    <div class="brand">
      <div class="brand-mark">O</div>
      <div>
        <strong>OMO</strong>
        <span>{labels.brandSubtitle}</span>
      </div>
    </div>

    <nav class="nav-list">
      {#each consoleNavItems as item}
        {@const Icon = item.icon}
        <a class:active={isActive(item.href)} href={item.href}>
          <Icon size={18} strokeWidth={1.8} />
          <span>{labels[item.key]}</span>
        </a>
      {/each}
    </nav>
  </aside>

  <main class="workspace">
    <header class="topbar">
      <div>
        <p class="eyebrow">{eyebrow ?? labels.operationsConsole}</p>
        <h1>{title}</h1>
      </div>

      <div class="topbar-actions">
        {#if statusLabel}
          <div class:ready={statusReady} class="status-pill">
            <span></span>
            {statusLabel}
          </div>
        {/if}
        {#if actions}
          {@render actions()}
        {/if}
        <div class="console-preference-controls" aria-label={labels.interfacePreferences}>
          <button
            type="button"
            title={$preferences.language === 'zh-CN' ? labels.switchToEnglish : labels.switchToChinese}
            aria-label={$preferences.language === 'zh-CN' ? labels.switchToEnglish : labels.switchToChinese}
            onclick={toggleLanguage}
          >
            <Languages size={16} />
            <span>{labels.currentLanguageShort}</span>
          </button>
          <button
            type="button"
            title={$preferences.theme === 'light' ? labels.switchToDark : labels.switchToLight}
            aria-label={$preferences.theme === 'light' ? labels.switchToDark : labels.switchToLight}
            onclick={toggleTheme}
          >
            {#if $preferences.theme === 'light'}
              <Sun size={16} />
            {:else}
              <Moon size={16} />
            {/if}
          </button>
        </div>
      </div>
    </header>

    {@render children()}
  </main>
</div>
