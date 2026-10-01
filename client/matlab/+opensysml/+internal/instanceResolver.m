function resolve = instanceResolver(instances)
%INSTANCERESOLVER Resolve an instance id when its graph carries that object.

    resolve = @(id) resolveOne(instances, id);
end

function value = resolveOne(instances, id)
    key = sprintf('%d', int64(id));
    if isKey(instances, key)
        value = instances(key);
    else
        value = int64(id);
    end
end
