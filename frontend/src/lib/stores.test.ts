import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { toasts, pushToast, dismissToast } from './stores';

describe('toast store', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    toasts.set([]);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('adds a toast with its kind and text', () => {
    pushToast('ok', 'saved');
    const list = get(toasts);
    expect(list).toHaveLength(1);
    expect(list[0].kind).toBe('ok');
    expect(list[0].text).toBe('saved');
  });

  it('auto-dismisses a success toast after the timeout', () => {
    pushToast('ok', 'saved');
    expect(get(toasts)).toHaveLength(1);

    vi.advanceTimersByTime(2999);
    expect(get(toasts)).toHaveLength(1);

    vi.advanceTimersByTime(1);
    expect(get(toasts)).toHaveLength(0);
  });

  it('keeps an error toast until it is dismissed', () => {
    pushToast('err', 'start failed');

    vi.advanceTimersByTime(60_000);
    expect(get(toasts)).toHaveLength(1);

    dismissToast(get(toasts)[0].id);
    expect(get(toasts)).toHaveLength(0);
  });

  it('stacks toasts and gives each a unique id', () => {
    pushToast('err', 'first');
    pushToast('err', 'second');

    const list = get(toasts);
    expect(list.map((t) => t.text)).toEqual(['first', 'second']);
    expect(list[0].id).not.toBe(list[1].id);
  });

  it('dismisses only the requested toast', () => {
    const first = pushToast('err', 'first');
    pushToast('err', 'second');

    dismissToast(first);

    const list = get(toasts);
    expect(list).toHaveLength(1);
    expect(list[0].text).toBe('second');
  });

  it('ignores a dismiss for an unknown id', () => {
    pushToast('err', 'only');
    dismissToast(9999);
    expect(get(toasts)).toHaveLength(1);
  });

  it('does not stack a repeated identical message', () => {
    // A refresh that keeps failing fires the same error every couple of
    // seconds for as long as a workspace is running.
    for (let i = 0; i < 30; i++) pushToast('err', 'refresh: boom');

    const list = get(toasts);
    expect(list).toHaveLength(1);
    expect(list[0].text).toBe('refresh: boom');
  });

  it('returns the existing id for a repeated message', () => {
    const first = pushToast('err', 'same');
    const second = pushToast('err', 'same');
    expect(second).toBe(first);
  });

  it('distinguishes messages that differ only by kind', () => {
    pushToast('ok', 'thing');
    pushToast('err', 'thing');
    expect(get(toasts)).toHaveLength(2);
  });

  it('caps the stack at the newest few distinct messages', () => {
    for (let i = 0; i < 10; i++) pushToast('err', `failure ${i}`);

    const list = get(toasts);
    expect(list).toHaveLength(4);
    expect(list.map((t) => t.text)).toEqual(['failure 6', 'failure 7', 'failure 8', 'failure 9']);
  });

  it('keeps a repeated success on its original timer', () => {
    pushToast('ok', 'saved');
    vi.advanceTimersByTime(2000);
    pushToast('ok', 'saved');

    // The repeat reuses the showing toast rather than extending it.
    vi.advanceTimersByTime(1000);
    expect(get(toasts)).toHaveLength(0);
  });

  it('does not remove a replacement toast that reuses a slot', () => {
    // A success toast's pending timer must not evict whatever is in the
    // list when it fires, only the toast it was created for.
    const okId = pushToast('ok', 'saved');
    dismissToast(okId);
    pushToast('err', 'later failure');

    vi.advanceTimersByTime(10_000);

    const list = get(toasts);
    expect(list).toHaveLength(1);
    expect(list[0].text).toBe('later failure');
  });
});
