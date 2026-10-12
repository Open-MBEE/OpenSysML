function result = renderView(model, viewName, varargin)
%RENDERVIEW Render a named view as machine-readable diagram data.

    ports = 'minimal';
    for i = 1:2:numel(varargin)
        if i == numel(varargin)
            opensysml.internal.raise('opensysml:argument', ...
                'renderView options must be name-value pairs');
        end
        if strcmp(varargin{i}, 'ports'), ports = char(varargin{i+1});
        else
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('renderView has no option ''%s''', varargin{i}));
        end
    end
    if ~any(strcmp(ports, {'minimal', 'full'}))
        opensysml.internal.raise('opensysml:argument', ...
            'ports must be ''minimal'' or ''full''');
    end
    capability = {'render_view'};
    model.connection.requireAll(capability);
    request = struct('modelHash', model.hash, 'view', char(viewName));
    if strcmp(ports, 'full'), request.ports = 'full'; end
    answer = opensysml.call(model.connection, 'RenderView', request, capability);
    result = opensysml.RenderedView(answer);
end
