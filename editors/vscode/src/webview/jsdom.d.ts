// What the canvas tests use of jsdom: a document to draw into.
declare module "jsdom" {
  export class JSDOM {
    constructor(html: string);
    readonly window: Window & typeof globalThis;
  }
}
