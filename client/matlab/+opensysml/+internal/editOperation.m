function operation = editOperation(arm, fields)
%EDITOPERATION Wrap one edit payload in its protobuf oneof arm.

    operation = struct();
    operation.(arm) = fields;
end
