"""Integration tests for the shape a performed action's pins hold, against a real service.

Without a service they skip, as a developer with no binary wants; with
$OPENSYSML_REQUIRE_SERVICE set, as CI sets it, an absent service fails instead.
"""

import pytest

from opensysml import Connection
from tests.service_gate import skip_or_fail_without_service

MODEL = '''
package Pins {
    private import ScalarValues::*;
    action def Shape {
        in value : Integer[1];
        out one : Integer[1];
        out bare : Integer;
        out many : Integer[0..*];
        first start;
        then action s { assign one := value; assign bare := value; assign many := (value, value + 1); }
        then done;
    }
}
'''


@pytest.mark.integration
class TestPerformedActionPinShape:
    """A pin is held in the shape its multiplicity gives it on the wire."""

    def setup_method(self):
        """Check if service is running."""
        import grpc
        try:
            self.conn = Connection(auto_start=False)
            from opensysml.proto import sysml_pb2
            self.conn._stub.GetDiagnostics(sysml_pb2.DiagnosticsRequest(model_hash=""))
        except grpc.RpcError as e:
            if e.code() == grpc.StatusCode.NOT_FOUND:
                return  # the service is up; no model has that hash
            self.conn = None
            skip_or_fail_without_service(
                f"the sysml-grpc service on localhost:50051 answered {e.code()}"
            )
        except Exception as e:
            self.conn = None
            skip_or_fail_without_service(
                f"no sysml-grpc service could be reached on localhost:50051 ({e})"
            )

    def teardown_method(self):
        if getattr(self, 'conn', None) is not None:
            self.conn.close()

    def test_scalar_bare_and_multi_valued_pins(self):
        """A [1] pin is its one value, a bare pin a one-element sequence, a
        [0..*] pin the sequence it holds."""
        model = self.conn.load_from_content(MODEL)
        outputs = model.execute_action("Pins::Shape", inputs={"value": 5})
        assert outputs["one"] == 5
        assert outputs["bare"] == [5]
        assert outputs["many"] == [5, 6]
