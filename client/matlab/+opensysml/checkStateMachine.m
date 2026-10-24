function result = checkStateMachine(model, stateMachineId, varargin)
%CHECKSTATEMACHINE Search every schedule of a state machine; see opensysml.checkAction.

    defaults = struct('performer', '', 'properties', {{}}, 'diverge', {{}}, 'freeInputs', {{}}, ...
        'assume', {{}}, 'witness', false, 'depth', 0, 'states', 0, 'unroll', 0, 'timeout', 0, ...
        'engine', '');
    options = opensysml.internal.nameValueOptions(defaults, varargin, 'checkStateMachine');
    result = opensysml.internal.checkBehavior(model, '', char(stateMachineId), struct(), options);
end
