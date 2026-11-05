namespace OpenMBEE.OpenSysML.Internal;

/// <summary>Finds an object by the id the service gave it among those an answer reported.</summary>
internal static class InstanceSearch
{
    /// <summary>The object of an id; null for 0, which names no object, and for an id not among them.</summary>
    internal static Instance? Find(ValueList<Instance> instances, long instanceId)
    {
        if (instanceId == 0)
        {
            return null;
        }

        foreach (var instance in instances)
        {
            if (instance.Id == instanceId)
            {
                return instance;
            }
        }

        return null;
    }
}
