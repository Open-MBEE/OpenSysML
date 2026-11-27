import { beforeEach, describe, expect, it } from 'vitest';
import { lastSeenHead, rememberedTarget, rememberPublication } from './publishMemory';

describe('publishMemory', () => {
  beforeEach(() => window.localStorage.clear());

  it('remembers nothing before a publication', () => {
    expect(rememberedTarget('ctx')).toEqual({ project: '', branch: '' });
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBeNull();
  });

  it('offers the target as it was asked for and knows the head of the branch it resolved to', () => {
    rememberPublication('ctx', { project: 'p1', branch: '' }, 'main', 'c1');

    expect(rememberedTarget('ctx')).toEqual({ project: 'p1', branch: '' });
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBe('c1');
    expect(lastSeenHead('ctx', { project: 'p1', branch: 'main' })).toBe('c1');
  });

  it('keeps the head of a branch after another branch or project is published', () => {
    rememberPublication('ctx', { project: 'p1', branch: '' }, 'main', 'c1');
    rememberPublication('ctx', { project: 'p1', branch: 'dev' }, 'dev', 'd1');
    rememberPublication('ctx', { project: 'p2', branch: 'main' }, 'main', 'e1');

    expect(rememberedTarget('ctx')).toEqual({ project: 'p2', branch: 'main' });
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBe('c1');
    expect(lastSeenHead('ctx', { project: 'p1', branch: 'main' })).toBe('c1');
    expect(lastSeenHead('ctx', { project: 'p1', branch: 'dev' })).toBe('d1');
    expect(lastSeenHead('ctx', { project: 'p2', branch: 'main' })).toBe('e1');
  });

  it('knows no head for a branch, project or editing context never published to', () => {
    rememberPublication('ctx', { project: 'p1', branch: 'dev' }, 'dev', 'c1');

    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBeNull();
    expect(lastSeenHead('ctx', { project: 'p1', branch: 'main' })).toBeNull();
    expect(lastSeenHead('ctx', { project: 'p2', branch: 'dev' })).toBeNull();
    expect(lastSeenHead('other', { project: 'p1', branch: 'dev' })).toBeNull();
  });

  it('keeps the head of a branch with no commit apart from a branch never seen', () => {
    rememberPublication('ctx', { project: 'p1', branch: 'main' }, 'main', '');

    expect(lastSeenHead('ctx', { project: 'p1', branch: 'main' })).toBe('');
  });

  it('ignores a stored value it did not write', () => {
    window.localStorage.setItem('opensysml.publish.ctx', '{"last": {"project": 7}}');
    expect(rememberedTarget('ctx')).toEqual({ project: '', branch: '' });
    window.localStorage.setItem('opensysml.publish.ctx', 'not json');
    expect(rememberedTarget('ctx')).toEqual({ project: '', branch: '' });
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBeNull();
  });
});
