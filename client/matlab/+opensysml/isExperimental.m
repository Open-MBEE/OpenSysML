function tf = isExperimental(fromFormat, toFormat)
%ISEXPERIMENTAL Whether a conversion uses an experimental mapping.

    fromFormat = lower(char(fromFormat));
    toFormat = lower(char(toFormat));
    rdf = {'ttl', 'turtle', 'rdf'};
    apiJson = {'api-json', 'json'};
    xmi = {'xmi', 'uml', 'mdzip'};
    tf = any(strcmp(fromFormat, rdf)) || any(strcmp(toFormat, rdf)) || ...
        any(strcmp(fromFormat, apiJson)) || any(strcmp(toFormat, apiJson)) || ...
        any(strcmp(fromFormat, xmi));
end
