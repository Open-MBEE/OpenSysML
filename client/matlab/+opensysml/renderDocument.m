function text = renderDocument(model, documentId, varargin)
%RENDERDOCUMENT Render a named document to Markdown, HTML or AsciiDoc.

    form = 'markdown';
    for i = 1:2:numel(varargin)
        if i == numel(varargin)
            opensysml.internal.raise('opensysml:argument', ...
                'renderDocument options must be name-value pairs');
        end
        if strcmp(varargin{i}, 'form'), form = char(varargin{i+1});
        else
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('renderDocument has no option ''%s''', varargin{i}));
        end
    end
    if ~any(strcmp(form, {'markdown', 'html', 'asciidoc'}))
        opensysml.internal.raise('opensysml:argument', ...
            'form must be ''markdown'', ''html'' or ''asciidoc''');
    end
    capabilities = {'render_document'};
    if ~strcmp(form, 'markdown'), capabilities{end+1} = ['render_document_' form]; end
    model.connection.requireAll(capabilities);
    request = struct('modelHash', model.hash, 'documentId', char(documentId));
    if ~strcmp(form, 'markdown'), request.form = form; end
    answer = opensysml.call(model.connection, 'RenderDocument', request, capabilities);
    text = fieldOr(answer, form, '');
end

function value = fieldOr(record, name, fallback)
    if isfield(record, name), value = record.(name); else, value = fallback; end
end
