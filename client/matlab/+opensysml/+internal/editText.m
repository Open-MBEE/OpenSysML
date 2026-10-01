function value = editText(label, value, optional, wording)
%EDITTEXT Validate and normalize notation text used by the editor.

    if nargin < 3, optional = false; end
    if nargin < 4, wording = 'notation text'; end
    if optional && isempty(value), return; end
    if isstring(value) && isscalar(value), value = char(value); end
    if ~ischar(value)
        opensysml.internal.raise('opensysml:argument', sprintf('%s must be %s, not %s', ...
            label, wording, typeName(value)));
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
