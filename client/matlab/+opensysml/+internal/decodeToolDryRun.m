function preview = decodeToolDryRun(raw)
%DECODETOOLDRYRUN Read a ToolDryRun message into a struct, field by field.

    preview = struct('action', text(raw, 'action'), 'tool', text(raw, 'tool'), ...
        'version', text(raw, 'version'), 'manifest', text(raw, 'manifest'), ...
        'executable', text(raw, 'executable'), 'protocol', text(raw, 'protocol'), ...
        'args', {texts(raw, 'args')}, 'env', {texts(raw, 'env')}, ...
        'cwd', text(raw, 'cwd'), 'stdin', text(raw, 'stdin'), ...
        'stdinData', text(raw, 'stdinData'), 'inputFile', [], ...
        'outputDir', false, 'inputs', {{}}, 'outputs', {{}}, ...
        'replySource', text(raw, 'replySource'), 'lines', {texts(raw, 'lines')});
    if isfield(raw, 'inputFile') && isstruct(raw.inputFile)
        preview.inputFile = struct('name', text(raw.inputFile, 'name'), ...
            'format', text(raw.inputFile, 'format'), 'data', text(raw.inputFile, 'data'));
    end
    if isfield(raw, 'outputDir'), preview.outputDir = logical(raw.outputDir); end
    inputs = records(raw, 'inputs');
    for i = 1:numel(inputs)
        preview.inputs{end+1} = struct('variable', text(inputs{i}, 'variable'), ...
            'parameter', text(inputs{i}, 'parameter'), 'value', text(inputs{i}, 'value'), ...
            'unit', text(inputs{i}, 'unit'));
    end
    outputs = records(raw, 'outputs');
    for i = 1:numel(outputs)
        preview.outputs{end+1} = struct('variable', text(outputs{i}, 'variable'), ...
            'parameter', text(outputs{i}, 'parameter'));
    end
end

function value = text(raw, name)
    value = '';
    if isstruct(raw) && isfield(raw, name) && ~isempty(raw.(name)), value = char(raw.(name)); end
end

function values = texts(raw, name)
    values = {};
    if ~(isstruct(raw) && isfield(raw, name)), return; end
    items = raw.(name);
    if ischar(items), values = {items};
    elseif iscell(items), values = cellfun(@char, items(:)', 'UniformOutput', false);
    end
end

function items = records(raw, name)
    items = {};
    if ~(isstruct(raw) && isfield(raw, name)), return; end
    items = raw.(name);
    if isstruct(items), items = num2cell(items(:)'); end
    if ~iscell(items), items = {}; end
end
