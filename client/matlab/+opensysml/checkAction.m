function result = checkAction(model, actionId, varargin)
%CHECKACTION Search every schedule of an action, as sysml -check-property does.
%   RESULT = CHECKACTION(MODEL, ACTIONID, 'properties', {...}, ...) answers a
%   struct with verdict ('holds', 'fails' or 'undecided'), lines (the report),
%   search (states, moves, maxDepth, boundsHit, ...), violations, divergences,
%   finals, claim, freeInputs, assumptions, witness, contrast, standing and
%   reason. Options: inputs, performer, properties, diverge,
%   freeInputs, assume, witness, depth, states, unroll, timeout, engine.

    options = opensysml.internal.nameValueOptions(checkDefaults(), varargin, 'checkAction');
    encoded = opensysml.internal.encodeNamedArguments(options.inputs, model.connection, ...
        'checkAction inputs');
    result = opensysml.internal.checkBehavior(model, char(actionId), '', encoded, options);
end

function defaults = checkDefaults()
    defaults = struct('inputs', struct(), 'performer', '', 'properties', {{}}, 'diverge', {{}}, ...
        'freeInputs', {{}}, 'assume', {{}}, 'witness', false, 'depth', 0, 'states', 0, ...
        'unroll', 0, 'timeout', 0, 'engine', '');
end
