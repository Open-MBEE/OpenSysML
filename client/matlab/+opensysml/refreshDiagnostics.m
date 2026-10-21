function diags = refreshDiagnostics(model, varargin)
%REFRESHDIAGNOSTICS Ask the service for the model's diagnostics now, under a lint selection.
%   model.diagnostics is what the parse answered under the default selection;
%   refreshDiagnostics(model, 'disableLint', {'undeclared-signal'}) leaves those
%   lints out and 'enableLint' keeps opt-in ones in, as sysml -disable-lint and
%   -enable-lint select them. A code naming no lint is refused naming the
%   lints there are.

    options = opensysml.internal.nameValueOptions(struct( ...
        'disableLint', {{}}, 'enableLint', {{}}), varargin, 'refreshDiagnostics');
    request = struct('modelHash', model.hash, ...
        'disableLint', {codes(options.disableLint, 'disableLint')}, ...
        'enableLint', {codes(options.enableLint, 'enableLint')});
    raw = opensysml.internal.checkError(opensysml.call(model.connection, ...
        'GetDiagnostics', request), 'GetDiagnostics');
    diags = {};
    if isfield(raw, 'diagnostics')
        diags = opensysml.internal.decodeDiagnostics(raw.diagnostics);
    end
end

function values = codes(raw, label)
    if isempty(raw), values = {};
    elseif ischar(raw), values = {raw};
    elseif iscellstr(raw), values = reshape(raw, 1, []);
    elseif exist('isstring', 'builtin') && isstring(raw), values = cellstr(reshape(raw, 1, []));
    else
        opensysml.internal.raise('opensysml:argument', ...
            sprintf('refreshDiagnostics %s must be lint codes', label));
    end
end
