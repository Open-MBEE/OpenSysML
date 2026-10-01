function inst = instantiate(model, typeId)
%INSTANTIATE Build an instance and its reachable feature-value graph.

    answer = opensysml.internal.checkError(opensysml.call(model.connection, 'Instantiate', ...
        struct('modelHash', model.hash, 'symbolId', char(typeId))), 'Instantiate');
    if ~isfield(answer, 'instance')
        opensysml.internal.raise('opensysml:diagnostics:execution', ...
            'Instantiate carried no instance');
    end
    graph = {};
    if isfield(answer, 'instances'), graph = answer.instances; end
    [decoded, instances] = opensysml.internal.decodeInstances(model.connection, graph);
    id = opensysml.parseInt64(answer.instance.id);
    key = sprintf('%d', id);
    if isKey(instances, key)
        inst = instances(key);
    else
        [decoded, instances] = opensysml.internal.decodeInstances(model.connection, answer.instance);
        inst = decoded{1};
    end
    inst.instances = instances;
    if ~isfield(inst, 'feature_values')
        inst.feature_values = containers.Map('KeyType', 'char', 'ValueType', 'any');
    end
    if ~isKey(instances, key)
        instances(key) = inst;
    end
end
