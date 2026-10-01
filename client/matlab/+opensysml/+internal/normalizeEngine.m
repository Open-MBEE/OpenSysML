function value = normalizeEngine(engine)
%NORMALIZEENGINE Spell automatic engine selection as the empty wire value.

    if isempty(engine), value = ''; return; end
    value = char(engine);
    if strcmp(value, 'auto'), value = ''; end
end
