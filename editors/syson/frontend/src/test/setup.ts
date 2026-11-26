import { afterEach, expect } from 'vitest';
import * as matchers from '@testing-library/jest-dom/matchers';
import { cleanup } from '@testing-library/react';

// Node 25 and later define a localStorage of their own, which Vitest leaves in place of the DOM's.
const dom = (globalThis as unknown as { jsdom: { window: Window } }).jsdom;
Object.defineProperty(globalThis, 'localStorage', { configurable: true, get: () => dom.window.localStorage });

expect.extend(matchers);
afterEach(() => cleanup());
