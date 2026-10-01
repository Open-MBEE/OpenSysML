function message = missingCapabilityMessage(capability, service)
%MISSINGCAPABILITYMESSAGE Describe the unsupported capability and remedy.

    remedy = sprintf(['run a sysml-grpc whose GetServerInfo reports ''%s'': build one with `make build-grpc` and start it yourself, ' ...
        'or point $OPENSYSML_GRPC_BINARY at a release that has it'], capability);
    message = sprintf(['the sysml-grpc service does not support the ''%s'' capability, which this operation requires.\n' ...
        '  service: %s\n  fix:     %s'], capability, service, remedy);
end
