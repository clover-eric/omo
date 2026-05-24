import { describe, expect, it } from 'vitest';
import { formatBootstrapState } from './status';

describe('formatBootstrapState', () => {
  it('formats the initial state', () => {
    expect(formatBootstrapState('UNINITIALIZED')).toBe('待初始化');
  });
});
