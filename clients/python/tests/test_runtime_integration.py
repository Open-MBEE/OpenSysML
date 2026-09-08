"""Integration tests for runtime operations against real service.

Without a service they skip, as a developer with no binary wants; with
$OPENSYSML_REQUIRE_SERVICE set, as CI sets it, an absent service fails instead.
"""
import pytest
from opensysml import Connection
from opensysml.errors import ExecutionError
from tests.service_gate import skip_or_fail_without_service

@pytest.mark.integration
class TestRuntimeIntegration:
    """Integration tests requiring live sysml-grpc service."""
    
    def setup_method(self):
        """Check if service is running."""
        import grpc
        try:
            self.conn = Connection(auto_start=False)
            # Probe health
            from opensysml.proto import sysml_pb2
            req = sysml_pb2.DiagnosticsRequest(model_hash="")
            self.conn._stub.GetDiagnostics(req)
        except grpc.RpcError as e:
            # NOT_FOUND is expected for invalid hash - means server is working
            if e.code() == grpc.StatusCode.NOT_FOUND:
                return  # Service is healthy, self.conn already set
            self.conn = None  # Failed, clear for teardown safety
            skip_or_fail_without_service(
                f"the sysml-grpc service on localhost:50051 answered {e.code()}"
            )
        except Exception as e:
            self.conn = None  # Failed, clear for teardown safety
            skip_or_fail_without_service(
                f"no sysml-grpc service could be reached on localhost:50051 ({e})"
            )
    
    def teardown_method(self):
        """Clean up connection after each test."""
        if hasattr(self, 'conn'):
            self.conn.close()
    
    def test_eval_arithmetic(self):
        """Test evaluating simple arithmetic expression."""
        # Parse a model with a calc
        src = 'package Test { calc result { 2 + 2 } }'
        model = self.conn.load_from_content(src)
        
        # Evaluate expression
        result = self.conn.eval("2 + 2", model.hash)
        assert result == 4
    
    def test_eval_boolean(self):
        """Test evaluating boolean expression."""
        src = 'package Test { }'
        model = self.conn.load_from_content(src)
        
        result = self.conn.eval("true and false", model.hash)
        assert result is False
    
    def test_instantiate_simple_part(self):
        """Test instantiating a part definition."""
        src = '''
        package Test {
            part def SimplePart {
                attribute mass : Integer = 100;
            }
        }
        '''
        model = self.conn.load_from_content(src)
        
        # Instantiate
        instance = self.conn.instantiate("Test::SimplePart", model.hash)
        
        assert instance is not None
        assert instance.type_symbol_id == "Test::SimplePart"
        assert instance.id > 0
    
    def test_instantiate_returns_pythonic_values(self):
        """Slot values arrive as Python scalars, not protobuf messages."""
        src = '''
        package Demo {
            part def Engine {
                attribute power = 300.0;
            }
            part def Vehicle {
                attribute mass = 1500.0;
                part engine : Engine;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        vehicle = self.conn.instantiate("Demo::Vehicle", model.hash)

        assert vehicle.mass == 1500.0
        assert vehicle["mass"] == 1500.0
        # Raw protobuf stays reachable.
        assert vehicle.get_feature("mass").materialized is True

    def test_instantiate_resolves_nested_instances(self):
        """A part slot resolves to a nested Instance, not a bare id."""
        from opensysml.instance import Instance

        src = '''
        package Demo {
            part def Engine {
                attribute power = 300.0;
            }
            part def Vehicle {
                attribute mass = 1500.0;
                part engine : Engine;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        vehicle = self.conn.instantiate("Demo::Vehicle", model.hash)

        assert isinstance(vehicle.engine, Instance)
        assert vehicle.engine.type_symbol_id == "Demo::Engine"
        assert vehicle.engine.power == 300.0

    def test_exhibited_state_values_read_through_the_occurrence(self):
        """Machine-owned values are readable through the exhibited state usage."""
        src = '''
        package Demo {
            state def Counting {
                attribute count : Integer = 0;
                entry; then active;
                state active {
                    entry action increment { assign count := count + 1; }
                }
            }
            part def Counter {
                attribute count : Integer = 10;
                exhibit state modes : Counting;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        counter = self.conn.instantiate("Demo::Counter", model.hash)

        assert counter.count == 10
        assert counter.modes.count == 1

    def test_performed_action_values_read_through_the_occurrence(self):
        """Action-owned values are readable through the perform usage."""
        src = '''
        package Demo {
            action def Bump {
                attribute count : Integer = 0;
                out attribute n : Integer;
                action step {
                    assign count := count + 1;
                    assign n := 42;
                }
                first step;
            }
            part def Host {
                attribute count : Integer = 10;
                perform action work : Bump;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        host = self.conn.instantiate("Demo::Host", model.hash)

        assert host.count == 10
        assert host.work.count == 1
        assert host.work.n == 42

    def test_instantiate_cyclic_attribute_reports_error(self):
        """A cyclic derived attribute surfaces as FeatureValueError, never as None."""
        from opensysml.errors import FeatureValueError

        src = '''
        package Demo {
            part def Cyclic {
                attribute a = b + 1.0;
                attribute b = a + 1.0;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        inst = self.conn.instantiate("Demo::Cyclic", model.hash)

        with pytest.raises(FeatureValueError, match="cyclic"):
            inst.a
        assert isinstance(inst.features["a"], FeatureValueError)

    def test_enum_typed_slot_and_eval_return_the_literal(self):
        """An enumeration literal reaches the client as the literal it is."""
        from opensysml import EnumLiteral

        src = '''
        package D {
            enum def Color { red; green; blue; }
            part def Car {
                attribute c : Color = Color::red;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        car = self.conn.instantiate("D::Car", model.hash)
        red = EnumLiteral("D::Color::red", "D::Color", "Color::red")
        assert car.c == red
        assert self.conn.eval("D::Color::red", model.hash) == red
        assert self.conn.eval("D::Color::green", model.hash) != red

    def test_service_reports_the_diagnostic_codes_capability(self):
        """The service says it populates Diagnostic.code, and does."""
        from opensysml.capabilities import CAPABILITY_DIAGNOSTIC_CODES

        assert self.conn.server_info().has(CAPABILITY_DIAGNOSTIC_CODES)
        model = self.conn.load_from_content("package P { part def W { part hub : Missing; } }")
        assert "unresolved" in {d.code for d in model.diagnostics}

    def test_service_reports_the_final_time_capability(self):
        """The service says it reports the clock a run ended at, and does."""
        from opensysml.capabilities import CAPABILITY_FINAL_TIME

        assert self.conn.server_info().has(CAPABILITY_FINAL_TIME)
        src = '''
        package Test {
            private import ScalarValues::*;
            state Timer {
                attribute fired : Integer = 0;
                entry; then armed;
                state armed;
                transition armed then done accept after 3 [SI::s] do assign fired := fired + 1;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        result = self.conn.execute_state("Test::Timer", model.hash)
        assert result["final_context"]["fired"] == 1
        assert result["final_time"] == 3.0

    def test_service_reports_the_enum_values_capability(self):
        """The wire form is a contract, so the service says it honours it."""
        from opensysml.capabilities import CAPABILITY_ENUM_VALUES

        assert self.conn.server_info().has(CAPABILITY_ENUM_VALUES)

    def test_a_valueless_feature_of_a_value_type_crosses_as_unset(self):
        """The service says it sends the unset arm, and does."""
        from opensysml.capabilities import CAPABILITY_UNSET_VALUE
        from opensysml.values import UNSET

        assert self.conn.server_info().has(CAPABILITY_UNSET_VALUE)

        src = '''
        package P {
            private import ScalarValues::*;
            part def Q {
                attribute d : Real;
                attribute k : Real = 2.0;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        q = self.conn.instantiate("P::Q", model.hash)
        assert q.d is UNSET
        assert q.k == 2.0

    def test_symbol_attributes_and_parts(self):
        """Symbol filtering works against the kinds the service really emits."""
        src = '''
        package Demo {
            part def Engine {
                attribute power = 300.0;
            }
            part def Vehicle {
                attribute mass = 1500.0;
                part engine : Engine;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        vehicle = model.find("Vehicle")
        assert vehicle is not None
        assert [a.name for a in vehicle.attributes()] == ["mass"]
        assert [p.name for p in vehicle.parts()] == ["engine"]
        assert vehicle.get_attr("mass").name == "mass"

    def test_eval_invalid_expression_raises(self):
        """Test that invalid expression raises ExecutionError."""
        src = 'package Test { }'
        model = self.conn.load_from_content(src)
        
        with pytest.raises(ExecutionError):
            self.conn.eval("invalid syntax (((", model.hash)
    
    def test_instantiate_nonexistent_symbol_raises(self):
        """Test that instantiating missing symbol raises ExecutionError."""
        src = 'package Test { }'
        model = self.conn.load_from_content(src)
        
        with pytest.raises(ExecutionError, match="not found"):
            self.conn.instantiate("Test::DoesNotExist", model.hash)
    
    def test_load_from_content_with_syntax_error(self):
        """Test that load_from_content returns diagnostics for invalid syntax."""
        src = 'package Test { invalid syntax ((( }'
        model = self.conn.load_from_content(src)
        assert len(model.diagnostics) > 0

    def test_execute_action_binds_inputs(self):
        """Supplied inputs must flow into the action and affect its outputs.

        The action seeds attribute `result` (default 0) then adds 5. Passing
        result=10 must yield 15, proving inputs are not discarded server-side.
        """
        src = '''
        package Test {
            action addFive {
                attribute result : Integer = 0;
                first start;
                action inner {
                    assign result := result + 5;
                }
                done;
                succession first start then inner;
                succession first inner then done;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        # Baseline: default 0 + 5 = 5.
        base = self.conn.execute_action("Test::addFive", model.hash)
        assert base.get("result") == 5

        # With input result=10 -> 10 + 5 = 15.
        out = self.conn.execute_action(
            "Test::addFive", model.hash, inputs={"result": 10}
        )
        assert out.get("result") == 15

    def test_execute_state_returns_real_trace(self):
        """states_visited must reflect the actual states, not a placeholder."""
        src = '''
        package Test {
            state Machine {
                entry; then init;
                state init;
                state Running;

                succession first init then Running;
                succession first Running then done;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        result = self.conn.execute_state("Test::Machine", model.hash)
        assert result["states_visited"] == ["init", "Running", "done"]

    def test_the_model_answers_the_hash_taking_calls_itself(self):
        """Every call taking a model_hash is reachable on the model it is about.

        A script that loaded a model has no reason to carry its hash back to the
        connection, so the model-level call must answer the same.
        """
        src = '''
        package Test {
            part def SimplePart {
                attribute mass : Integer = 100;
            }
            action addFive {
                attribute result : Integer = 0;
                first start;
                action inner {
                    assign result := result + 5;
                }
                done;
                succession first start then inner;
                succession first inner then done;
            }
            state Machine {
                entry; then init;
                state init;
                state Running;

                succession first init then Running;
                succession first Running then done;
            }
        }
        '''
        model = self.conn.load_from_content(src)

        assert model.instantiate("Test::SimplePart").mass == 100
        assert model.execute_action("Test::addFive") == self.conn.execute_action(
            "Test::addFive", model.hash
        )
        assert model.execute_action("Test::addFive", inputs={"result": 10})[
            "result"
        ] == 15
        assert model.execute_state("Test::Machine") == self.conn.execute_state(
            "Test::Machine", model.hash
        )
