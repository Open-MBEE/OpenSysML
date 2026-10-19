function result = renderView(model, viewName, varargin)
%RENDERVIEW Render a named view as machine-readable diagram data.
%   renderView(model, name, 'ports', 'full', 'show', {'edge-labels'}) names
%   the optional notation elements to draw besides those the view's
%   rendering shows ('all' for every one); result.shown lists the ones drawn.
%   'depth', n draws only n levels of nested elements below each exposed
%   element, overriding the view's own depth; 0 (the default) draws every
%   level the view allows.

    ports = 'minimal';
    show = {};
    depth = 0;
    for i = 1:2:numel(varargin)
        if i == numel(varargin)
            opensysml.internal.raise('opensysml:argument', ...
                'renderView options must be name-value pairs');
        end
        if strcmp(varargin{i}, 'ports'), ports = char(varargin{i+1});
        elseif strcmp(varargin{i}, 'show'), show = cellstr(varargin{i+1});
        elseif strcmp(varargin{i}, 'depth'), depth = varargin{i+1};
        else
            opensysml.internal.raise('opensysml:argument', ...
                sprintf('renderView has no option ''%s''', varargin{i}));
        end
    end
    if ~any(strcmp(ports, {'minimal', 'full'}))
        opensysml.internal.raise('opensysml:argument', ...
            'ports must be ''minimal'' or ''full''');
    end
    if ~(isnumeric(depth) && isscalar(depth) && depth >= 0 && depth == floor(depth))
        opensysml.internal.raise('opensysml:argument', ...
            'depth must be a non-negative integer');
    end
    capability = {'render_view'};
    model.connection.requireAll(capability);
    request = struct('modelHash', model.hash, 'view', char(viewName));
    if strcmp(ports, 'full'), request.ports = 'full'; end
    if ~isempty(show), request.show = reshape(show, 1, []); end
    if depth > 0, request.depth = double(depth); end
    answer = opensysml.call(model.connection, 'RenderView', request, capability);
    result = opensysml.RenderedView(answer);
end
