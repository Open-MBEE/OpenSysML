// What the webview tests use of jsdom: a document to draw into, at an origin the panel
// script posts to, with the animation frames its viewport work is scheduled in.
declare module "jsdom" {
  export class JSDOM {
    constructor(html: string, options?: { url?: string; pretendToBeVisual?: boolean });
    readonly window: Window & typeof globalThis;
  }
}
