"""Integration tests for a performed action's unbound input, against real service.

Without a service they skip, as a developer with no binary wants; with
$OPENSYSML_REQUIRE_SERVICE set, as CI sets it, an absent service fails instead.
"""
import pytest
from opensysml import Connection
from opensysml.values import Quantity
from tests.service_gate import skip_or_fail_without_service

MODEL = '''
package P {
    private import ScalarValues::*; private import ISQ::*; private import SI::*;
    action def ApplyHeat { in energy : ISQ::EnergyValue; }
    action def ToastBread { action applyHeat : ApplyHeat; }
    part def Toaster { attribute cycleTime : ISQ::DurationValue; perform action toastBread : ToastBread; }
    part slow : Toaster { attribute :>> cycleTime = 200.0 [SI::s]; }
}
'''

@pytest.mark.integration
class TestPerformedActionUnboundInput:
    """Integration tests requiring live sysml-grpc service."""

    def setup_method(self):
        """Check if service is running."""
        import grpc
        try:
            self.conn = Connection(auto_start=False)
            from opensysml.proto import sysml_pb2
            req = sysml_pb2.DiagnosticsRequest(model_hash="")
            self.conn._stub.GetDiagnostics(req)
        except grpc.RpcError as e:
            if e.code() == grpc.StatusCode.NOT_FOUND:
                return  # Service is healthy, self.conn already set
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
        """Clean up connection after each test."""
        if hasattr(self, 'conn'):
            self.conn.close()

    def test_eval_reads_past_a_failed_performance(self):
        """An unbound input ends the performance, not the performer's creation:
        the object's other features still evaluate."""
        model = self.conn.load_from_content(MODEL)
        result = model.eval("slow.cycleTime", context_symbol_id="P")
        assert isinstance(result, Quantity)
        assert result.magnitude == 200.0
        assert result.unit.text == "SI::s"

    def test_kindless_parameter_is_a_reference_usage(self):
        """A parameter declared with no kind keyword is a referenceUsage."""
        model = self.conn.load_from_content(MODEL)
        symbol = model._symbol_by_id("P::ApplyHeat::energy")
        assert symbol is not None
        assert symbol.kind == "referenceUsage"
