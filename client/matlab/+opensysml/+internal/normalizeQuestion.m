function value = normalizeQuestion(question)
%NORMALIZEQUESTION Spell the default evaluate question as the empty wire value.

    if isempty(question), value = ''; return; end
    value = char(question);
    if strcmp(value, 'evaluate'), value = ''; end
end
