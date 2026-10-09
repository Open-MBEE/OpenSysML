import { JupyterFrontEnd, JupyterFrontEndPlugin } from '@jupyterlab/application';
import { IEditorLanguageRegistry } from '@jupyterlab/codemirror';
import { LanguageSupport, StreamLanguage } from '@codemirror/language';

import { kerml, sysml } from './sysml';

/**
 * Registers the sysml and kerml CodeMirror languages, under the names, MIME
 * types and file extensions the sysml kernel's language_info reports, so
 * notebook cells, .sysml/.kerml files and ```sysml Markdown blocks are highlighted.
 */
const plugin: JupyterFrontEndPlugin<void> = {
  id: 'jupyterlab-opensysml:languages',
  description: 'SysML v2 and KerML syntax highlighting.',
  autoStart: true,
  requires: [IEditorLanguageRegistry],
  activate: (_app: JupyterFrontEnd, languages: IEditorLanguageRegistry): void => {
    languages.addLanguage({
      name: 'sysml',
      displayName: 'SysML v2',
      mime: 'text/x-sysml',
      extensions: ['sysml'],
      support: new LanguageSupport(StreamLanguage.define(sysml))
    });
    languages.addLanguage({
      name: 'kerml',
      displayName: 'KerML',
      mime: 'text/x-kerml',
      extensions: ['kerml'],
      support: new LanguageSupport(StreamLanguage.define(kerml))
    });
  }
};

export default plugin;
