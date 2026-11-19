import { beforeEach, describe, expect, it } from 'vitest';
import { lastSeenHead, rememberedTarget, rememberPublication } from './publishMemory';

describe('publishMemory', () => {
  beforeEach(() => window.localStorage.clear());

  it('remembers nothing before a publication', () => {
    expect(rememberedTarget('ctx')).toEqual({ project: '', branch: '' });
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBeNull();
  });

  it('offers the target as it was asked for and knows the head of the branch it resolved to', () => {
    rememberPublication('ctx', 'http://localhost:8083', 'p1', '', 'main', 'c1');

    expect(rememberedTarget('ctx')).toEqual({ project: 'p1', branch: '' });
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBe('c1');
    expect(lastSeenHead('ctx', { project: 'p1', branch: 'main' })).toBe('c1');
  });

  it('knows no head for another branch, project or editing context', () => {
    rememberPublication('ctx', 'http://localhost:8083', 'p1', 'dev', 'dev', 'c1');

    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBeNull();
    expect(lastSeenHead('ctx', { project: 'p1', branch: 'main' })).toBeNull();
    expect(lastSeenHead('ctx', { project: 'p2', branch: 'dev' })).toBeNull();
    expect(lastSeenHead('other', { project: 'p1', branch: 'dev' })).toBeNull();
  });

  it('keeps the head of a branch with no commit apart from a branch never seen', () => {
    rememberPublication('ctx', 'http://localhost:8083', 'p1', 'main', 'main', '');

    expect(lastSeenHead('ctx', { project: 'p1', branch: 'main' })).toBe('');
  });

  it('ignores a stored value it did not write', () => {
    window.localStorage.setItem('opensysml.publish.target.ctx', '{"project": 7}');
    expect(rememberedTarget('ctx')).toEqual({ project: '', branch: '' });
    window.localStorage.setItem('opensysml.publish.target.ctx', 'not json');
    expect(rememberedTarget('ctx')).toEqual({ project: '', branch: '' });
  });
});
