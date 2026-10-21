// What the webview tests use of jsdom: a document to draw into, at an origin the panel script posts to.
declare module "jsdom" {
  export class JSDOM {
    constructor(html: string, options?: { url?: string });
    readonly window: Window & typeof globalThis;
  }
}
