function value = conf_placeholders(value, modelHash, fixturesDir)
%CONF_PLACEHOLDERS ${model_hash} -> the parsed model's hash;
%${fixture:<name>} -> that fixture's source text.

    if isa(value, 'containers.Map')
        keys = value.keys;
        for i = 1:numel(keys)
            value(keys{i}) = conf_placeholders(value(keys{i}), modelHash, fixturesDir);
        end
    elseif iscell(value)
        for i = 1:numel(value)
            value{i} = conf_placeholders(value{i}, modelHash, fixturesDir);
        end
    elseif ischar(value)
        if strcmp(value, '${model_hash}')
            if isempty(modelHash)
                error('opensysml:conformance', 'request names ${model_hash} but scenario declares no model');
            end
            value = modelHash;
        elseif strncmp(value, '${fixture:', 10) && value(end) == '}'
            name = value(11:end-1);
            value = fileread(conf_fixture_path(fixturesDir, name));
        end
    end
end
