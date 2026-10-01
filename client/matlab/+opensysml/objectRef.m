function value = objectRef(varargin)
%OBJECTREF Reference an instantiated object by id, path, or both.

    id = int64(0);
    path = '';
    for i = 1:2:numel(varargin)
        if i == numel(varargin)
            opensysml.internal.raise('opensysml:argument', ...
                'objectRef options must be name-value pairs');
        end
        switch varargin{i}
            case 'id'
                reference = opensysml.instanceRef(varargin{i+1});
                id = reference.instanceRef;
            case 'path'
                path = char(varargin{i+1});
            otherwise
                opensysml.internal.raise('opensysml:argument', ...
                    sprintf('objectRef has no option ''%s''', varargin{i}));
        end
    end
    value = struct('type', 'object', 'id', id, 'path', path, ...
        'element', []);
end
