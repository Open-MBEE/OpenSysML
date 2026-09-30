function tf = conf_is_default(value)
%CONF_IS_DEFAULT The proto defaults: null, false, "", 0, "0", [], {}.

    tf = false;
    if isempty(value)
        tf = true; return;
    end
    if islogical(value) && ~value
        tf = true; return;
    end
    if ischar(value)
        if isempty(value), tf = true; return; end
        n = str2double(value);
        if ~isnan(n) && n == 0, tf = true; end
        return;
    end
    if isnumeric(value) && isscalar(value) && value == 0
        tf = true; return;
    end
    if iscell(value) && isempty(value)
        tf = true; return;
    end
    if isa(value, 'containers.Map') && value.Count == 0
        tf = true; return;
    end
end
