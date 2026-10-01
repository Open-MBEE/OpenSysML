function id = editTargetId(target)
%EDITTARGETID Return an editor target id from text or a Symbol.

    if isa(target, 'opensysml.Symbol')
        id = target.id;
    elseif ischar(target) || (isstring(target) && isscalar(target))
        id = char(target);
    else
        opensysml.internal.raise('opensysml:argument', sprintf( ...
            'target must be a symbol id (FQN) or a Symbol, not %s', typeName(target)));
    end
end

function name = typeName(value)
    if isempty(value), name = 'NoneType';
    elseif islogical(value), name = 'bool';
    elseif isnumeric(value) && isscalar(value) && isfinite(double(value)) && ...
            double(value) == fix(double(value)), name = 'int';
    elseif isnumeric(value), name = 'float';
    elseif iscell(value), name = 'list';
    elseif isstruct(value) || isa(value, 'containers.Map'), name = 'dict';
    else, name = class(value);
    end
end
