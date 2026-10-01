classdef SourceDocument
%SOURCEDOCUMENT One file or inline document to parse.

    properties
        path = []
        name = ''
        content = []
        language = []
    end

    methods
        function doc = SourceDocument(path, content, name, language)
            if nargin < 1, path = []; end
            if nargin < 2, content = []; end
            if nargin < 3, name = ''; end
            if nargin < 4, language = []; end
            doc.path = textOrEmpty(path, 'path');
            doc.content = textOrEmpty(content, 'content');
            doc.name = textOrEmpty(name, 'name');
            doc.language = textOrEmpty(language, 'language');
            hasPath = ischar(doc.path);
            hasContent = ischar(doc.content);
            if hasPath == hasContent
                opensysml.internal.raise('opensysml:argument', ...
                    'a SourceDocument is either a file path or inline content, not both');
            end
            if hasPath
                if isempty(doc.path)
                    opensysml.internal.raise('opensysml:argument', 'a file document needs a path');
                end
                if ~isempty(doc.name)
                    opensysml.internal.raise('opensysml:argument', ...
                        'a file document is named by its path; name applies to inline content');
                end
                if ischar(doc.language)
                    opensysml.internal.raise('opensysml:argument', ...
                        'a file extension says which language it is; language applies to inline content');
                end
            else
                if isempty(doc.name)
                    opensysml.internal.raise('opensysml:argument', ...
                        'inline content needs a name; diagnostics report it under that name');
                end
                if ischar(doc.language) && ~any(strcmp(doc.language, {'sysml', 'kerml'}))
                    opensysml.internal.raise('opensysml:argument', ...
                        'language must be ''sysml'' or ''kerml''');
                end
            end
        end

        function name = documentName(doc)
            if ~isempty(doc.path), name = doc.path;
            else, name = doc.name;
            end
        end
    end

    methods (Static)
        function doc = file(path)
            doc = opensysml.SourceDocument(path, [], '', []);
        end

        function doc = inline(name, content, language)
            if nargin < 3, language = []; end
            doc = opensysml.SourceDocument([], content, name, language);
        end
    end
end

function value = textOrEmpty(value, name)
    if isempty(value) && ~ischar(value)
        return;
    end
    if isStringValue(value) && isscalar(value), value = char(value); end
    if ~ischar(value)
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('%s must be text', name));
    end
end

function tf = isStringValue(value)
    tf = false;
    if exist('isstring', 'builtin') || exist('isstring', 'file')
        tf = isstring(value);
    end
end
