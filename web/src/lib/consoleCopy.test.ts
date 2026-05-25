import { describe, expect, it } from 'vitest';
import { consoleLabels } from './consoleCopy';

describe('consoleLabels', () => {
  it('shows the next language on the toggle', () => {
    expect(consoleLabels('zh-CN').nextLanguageShort).toBe('EN');
    expect(consoleLabels('en-US').nextLanguageShort).toBe('中文');
  });
});
