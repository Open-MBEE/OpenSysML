function request = lookupKind(connection, request, lookup)
%LOOKUPKIND State on a request what its one string names: 'qualified_name', 'element_id',
%or 'either', the legacy form, which a service without symbol_lookup_kind is asked in.

    if ~ischar(lookup) || ~any(strcmp(lookup, {'either', 'qualified_name', 'element_id'}))
        opensysml.internal.raise('opensysml:argument', ...
            'lookup is ''either'', ''qualified_name'' or ''element_id''');
    end
    if strcmp(lookup, 'either') || ~connection.hasCapability('symbol_lookup_kind')
        return;
    end
    if strcmp(lookup, 'qualified_name')
        request.lookup = 'LOOKUP_KIND_QUALIFIED_NAME';
    else
        request.lookup = 'LOOKUP_KIND_ELEMENT_ID';
    end
end
