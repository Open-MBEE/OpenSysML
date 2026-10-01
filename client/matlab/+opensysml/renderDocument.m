function text = renderDocument(model, documentId, varargin)
%RENDERDOCUMENT Render a named document to Markdown or HTML.

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
    if ~any(strcmp(form, {'markdown', 'html'}))
        opensysml.internal.raise('opensysml:argument', ...
            'form must be ''markdown'' or ''html''');
    end
    capabilities = {'render_document'};
    if strcmp(form, 'html'), capabilities{end+1} = 'render_document_html'; end
    model.connection.requireAll(capabilities);
    request = struct('modelHash', model.hash, 'documentId', char(documentId));
    if strcmp(form, 'html'), request.form = 'html'; end
    answer = opensysml.call(model.connection, 'RenderDocument', request, capabilities);
    if strcmp(form, 'html'), text = fieldOr(answer, 'html', '');
    else, text = fieldOr(answer, 'markdown', '');
    end
end

function value = fieldOr(record, name, fallback)
    if isfield(record, name), value = record.(name); else, value = fallback; end
end
