function tf = isV1(fromFormat)
%ISV1 Whether a format name is a SysML v1 form: xmi, uml or mdzip, in any case and padding.

    name = lower(strtrim(char(fromFormat)));
    tf = any(strcmp(name, {'xmi', 'uml', 'mdzip'}));
end
