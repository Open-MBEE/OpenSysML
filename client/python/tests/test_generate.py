"""Tests for the type mapping and emitter of opensysml.generate."""

import pytest

from opensysml.generate import (
    Definition,
    Feature,
    class_names,
    collect_definitions,
    element_type,
    feature_type,
    is_definition_kind,
    is_feature_kind,
    render_module,
)
from opensysml.typefacts import Multiplicity, Specialization, SymbolFacts, TypeFacts


def definition(fqn, kind="partDef", features=(), specializations=()):
    """Build a Definition from synthetic facts."""
    name = fqn.split("::")[-1]
    return Definition(
        SymbolFacts(id=fqn, name=name, kind=kind, specializations=tuple(specializations)),
        tuple(features),
    )


def feature(name, kind="attributeUsage", type_facts=None, multiplicity=None, owner="Demo::Vehicle"):
    """Build a Feature from synthetic facts."""
    return Feature(
        SymbolFacts(
            id=f"{owner}::{name}",
            name=name,
            kind=kind,
            type=type_facts,
            multiplicity=multiplicity,
        )
    )


class FakeSymbol:
    """A Symbol-shaped node for exercising the tree walk without a service."""

    def __init__(self, facts, children=()):
        self._facts = facts
        self._children = list(children)

    @property
    def id(self):
        return self._facts.id

    @property
    def kind(self):
        return self._facts.kind

    def facts(self):
        return self._facts

    def children(self):
        return self._children


class FakeInstance:
    """An Instance-shaped holder of slot values, for exercising generated accessors."""

    def __init__(self, slots, type_symbol_id=""):
        self._slots = dict(slots)
        # from_instance reads the reported type; empty means "not reported",
        # which it accepts rather than treating as a mismatch.
        self.type_symbol_id = type_symbol_id

    def __contains__(self, name):
        return name in self._slots

    def __getitem__(self, name):
        return self._slots[name]


def test_is_definition_kind_covers_service_kinds():
    """Every service kind ending in Def, plus metaclass, declares a type."""
    for kind in ("partDef", "attributeDef", "itemDef", "enumDef", "portDef", "metaclass"):
        assert is_definition_kind(kind)
    for kind in ("package", "partUsage", "attributeUsage", "connectorEnd"):
        assert not is_definition_kind(kind)


def test_is_feature_kind_excludes_behavioral_usages():
    """Structural usages become properties; behavioral ones are skipped."""
    for kind in ("attributeUsage", "partUsage", "itemUsage", "portUsage"):
        assert is_feature_kind(kind)
    for kind in ("actionUsage", "stateUsage", "calcUsage", "constraintUsage", "partDef"):
        assert not is_feature_kind(kind)


@pytest.mark.parametrize(
    "primitive,expected",
    [
        ("Boolean", "bool"),
        ("String", "str"),
        ("Natural", "int"),
        ("Integer", "int"),
        ("Rational", "float"),
        ("Real", "float"),
    ],
)
def test_element_type_primitives(primitive, expected):
    """Library scalars map to their Python counterparts."""
    mapped = element_type(TypeFacts(primitive=primitive), {})
    assert mapped.annotation == expected
    assert mapped.decoder == f"_t.as_{expected}"
    assert mapped.comment == ""


def test_element_type_generated_class():
    """A usage typed by a generated definition maps to that class."""
    mapped = element_type(TypeFacts(declared="Engine", resolved_id="Demo::Engine"), {"Demo::Engine": "Engine"})
    assert mapped.annotation == "Engine"
    assert mapped.decoder == "_t.as_typed(Engine)"


def test_element_type_scalar_definition_maps_to_its_scalar():
    """A definition that reduces to a library scalar holds that scalar, not an instance."""
    mapped = element_type(
        TypeFacts(declared="Celsius", resolved_id="Demo::Celsius", primitive="Real"),
        {"Demo::Celsius": "Celsius"},
    )
    assert mapped.annotation == "float"
    assert mapped.decoder == "_t.as_float"


def test_element_type_enumeration_is_a_literal():
    """An enumeration-typed usage holds a literal, not an instance of the enum def."""
    mapped = element_type(
        TypeFacts(declared="Color", resolved_id="D::Color", resolved_kind="enumDef"),
        {"D::Color": "Color"},
    )
    assert mapped.annotation == "_t.EnumLiteral"
    assert mapped.decoder == "_t.as_enum_literal"


def test_element_type_valued_enumeration_maps_to_its_scalar():
    """A literal declaring a value of its own evaluates to that value."""
    mapped = element_type(
        TypeFacts(
            declared="Code", resolved_id="D::Code", resolved_kind="enumDef",
            primitive="Integer",
        ),
        {"D::Code": "Code"},
    )
    assert mapped.annotation == "int"
    assert mapped.decoder == "_t.as_int"


def test_element_type_complex_is_a_python_complex():
    """A Complex slot holds one complex number, never two floats."""
    mapped = element_type(TypeFacts(primitive="Complex"), {})
    assert mapped.annotation == "complex"
    assert mapped.decoder == "_t.as_complex"


def test_element_type_unmapped_primitive_is_object():
    """Number has no sound Python type and says so."""
    mapped = element_type(TypeFacts(primitive="Number"), {})
    assert mapped.annotation == "object"
    assert "Number" in mapped.comment


def test_element_type_quantity_is_a_quantity_naming_its_unit():
    """A quantity maps to the Quantity class, whatever scalar it is written over."""
    mapped = element_type(TypeFacts(primitive="Real", quantity=True, unit="kg"), {})
    assert mapped.annotation == "_t.Quantity"
    assert mapped.decoder == "_t.as_quantity"
    assert "kg" in mapped.comment

    # A quantity value type says nothing about what was written for it: such a
    # slot may hold a plain number or a structured value.
    no_unit = element_type(TypeFacts(primitive="Real", quantity=True), {})
    assert no_unit.annotation == "object"
    assert no_unit.decoder == "_t.as_object"


def test_element_type_unresolved_and_untyped():
    """An unresolved or absent type maps to object, naming what was written."""
    unresolved = element_type(TypeFacts(declared="Missing"), {})
    assert unresolved.annotation == "object"
    assert "Missing" in unresolved.comment

    untyped = element_type(None, {})
    assert untyped.annotation == "object"
    assert untyped.comment


def test_element_type_type_without_generated_class():
    """A type resolved outside the generated set maps to object, naming the FQN."""
    mapped = element_type(TypeFacts(declared="Anything", resolved_id="Base::Anything"), {})
    assert mapped.annotation == "object"
    assert "Base::Anything" in mapped.comment


@pytest.mark.parametrize(
    "multiplicity,expected",
    [
        (None, "float"),
        (Multiplicity(lower="1", upper="1"), "float"),
        (Multiplicity(lower="0", upper="1"), "float | None"),
        (Multiplicity(lower="0", upper="*"), "list[float]"),
        (Multiplicity(lower="2", upper="4"), "list[float]"),
        (Multiplicity(lower="1", upper=""), "float"),
    ],
)
def test_feature_type_multiplicity(multiplicity, expected):
    """Multiplicity decides between a bare value, an option and a list."""
    mapped = feature_type(feature("mass", type_facts=TypeFacts(primitive="Real"), multiplicity=multiplicity), {})
    assert mapped.annotation == expected


def test_feature_named_like_a_typed_object_member_is_renamed():
    """A feature named `instance` must not shadow the accessor machinery it uses."""
    source = render_module(
        [
            definition(
                "Demo::Vehicle",
                features=[feature("instance", type_facts=TypeFacts(primitive="Real"))],
            )
        ]
    )
    assert "def instance_(self) -> float:" in source
    assert '_t.feature_value(self, "instance", _t.as_float)' in source

    namespace: dict = {}
    exec(compile(source, "generated", "exec"), namespace)
    assert namespace["Vehicle"].instance is not None


def test_feature_named_unchecked_does_not_shadow_the_escape_hatch():
    """A feature named `unchecked` must not hide the unchecked view classmethod."""
    source = render_module(
        [
            definition(
                "Demo::Vehicle",
                features=[feature("unchecked", type_facts=TypeFacts(primitive="Real"))],
            )
        ]
    )
    assert "def unchecked_(self) -> float:" in source

    namespace: dict = {}
    exec(compile(source, "generated", "exec"), namespace)
    vehicle = namespace["Vehicle"]
    view = vehicle.unchecked(FakeInstance({"unchecked": 1.5}))
    assert view.unchecked_ == 1.5


def test_unrestricted_name_with_quotes_is_escaped():
    """A SysML unrestricted name may carry quotes and backslashes; output must import."""
    source = render_module(
        [
            definition(
                'Demo::say "hi"\\x',
                features=[
                    feature(
                        'mass "kg"\\x',
                        type_facts=TypeFacts(primitive="Real"),
                        owner='Demo::say "hi"\\x',
                    )
                ],
            )
        ]
    )
    namespace: dict = {}
    exec(compile(source, "generated", "exec"), namespace)
    generated = namespace["say__hi__x"]
    assert generated.sysml_id == 'Demo::say "hi"\\x'
    assert generated.from_instance(FakeInstance({'mass "kg"\\x': 1500.0})).mass__kg__x == 1500.0


def test_class_names_disambiguate_collisions():
    """Two definitions of the same simple name both get qualified class names."""
    definitions = [definition("A::Thing"), definition("B::Thing"), definition("A::Other")]
    names = class_names(definitions)
    assert names == {"A::Thing": "A_Thing", "B::Thing": "B_Thing", "A::Other": "Other"}


def test_class_names_sanitize_identifiers():
    """A SysML name that is not a Python identifier is made into one."""
    names = class_names([definition("P::my part"), definition("P::class")])
    assert names["P::my part"] == "my_part"
    assert names["P::class"] == "class_"


def test_collect_definitions_walks_tree_and_sorts():
    """Definitions come back sorted by FQN with their structural features."""
    engine = FakeSymbol(SymbolFacts(id="Demo::Engine", name="Engine", kind="partDef"))
    mass = FakeSymbol(SymbolFacts(id="Demo::Vehicle::mass", name="mass", kind="attributeUsage"))
    constraint = FakeSymbol(SymbolFacts(id="Demo::Vehicle::ok", name="ok", kind="constraintUsage"))
    vehicle = FakeSymbol(
        SymbolFacts(id="Demo::Vehicle", name="Vehicle", kind="partDef"), [mass, constraint]
    )
    root = FakeSymbol(SymbolFacts(id="Demo", name="Demo", kind="package"), [vehicle, engine])

    definitions = collect_definitions(root)
    assert [d.id for d in definitions] == ["Demo::Engine", "Demo::Vehicle"]
    assert [f.name for f in definitions[1].features] == ["mass"]


def test_render_module_is_importable_and_deterministic(tmp_path):
    """The emitted module imports and rendering the same input twice is identical."""
    definitions = [
        definition("Demo::Engine", features=[feature("power", type_facts=TypeFacts(primitive="Real"), owner="Demo::Engine")]),
        definition(
            "Demo::Vehicle",
            features=[
                feature("engine", kind="partUsage", type_facts=TypeFacts(declared="Engine", resolved_id="Demo::Engine")),
                feature("mass", type_facts=TypeFacts(primitive="Real")),
            ],
        ),
    ]
    source = render_module(definitions)
    assert source == render_module(definitions)

    module_path = tmp_path / "demo_types.py"
    module_path.write_text(source)
    namespace: dict = {}
    exec(compile(source, str(module_path), "exec"), namespace)
    assert namespace["Vehicle"].sysml_id == "Demo::Vehicle"
    assert issubclass(namespace["Engine"], __import__("opensysml.typed", fromlist=["typed"]).TypedObject)


def test_render_module_emits_bases_before_subclasses():
    """A specialized definition is emitted after the definition it specializes."""
    car = definition("Demo::Car", specializations=[Specialization(kind="specializes", declared="Vehicle", target_id="Demo::Vehicle")])
    vehicle = definition("Demo::Vehicle")
    source = render_module([car, vehicle])
    assert source.index("class Vehicle(") < source.index("class Car(Vehicle)")


@pytest.mark.parametrize("kind", ["subsets", "redefines"])
def test_render_module_makes_every_generalization_edge_a_base(kind):
    """A subsets or redefines edge becomes a base class, like specializes does."""
    car = definition(
        "Demo::Car",
        specializations=[Specialization(kind=kind, declared="Vehicle", target_id="Demo::Vehicle")],
    )
    source = render_module([car, definition("Demo::Vehicle")])
    assert "class Car(Vehicle):" in source
    assert source.index("class Vehicle(") < source.index("class Car(Vehicle)")


def test_render_module_keeps_multiple_supertypes_in_declaration_order():
    """Several generalization edges become several bases, in declaration order."""
    hybrid = definition(
        "Demo::Hybrid",
        specializations=[
            Specialization(kind="specializes", declared="Vehicle", target_id="Demo::Vehicle"),
            Specialization(kind="subsets", declared="Electric", target_id="Demo::Electric"),
            # A repeated target is one base class, not two.
            Specialization(kind="redefines", declared="Vehicle", target_id="Demo::Vehicle"),
        ],
    )
    source = render_module([hybrid, definition("Demo::Vehicle"), definition("Demo::Electric")])
    assert "class Hybrid(Vehicle, Electric):" in source


def test_render_module_keeps_the_base_that_implies_the_other():
    """A base a sibling base already specializes is left implicit, not dropped for it."""
    vehicle = definition("Demo::Vehicle")
    electric = definition(
        "Demo::Electric",
        specializations=[Specialization(kind="specializes", declared="Vehicle", target_id="Demo::Vehicle")],
    )
    # Vehicle before Electric is unlinearizable as written: Electric is already a
    # Vehicle, so Electric alone preserves both relationships and its members.
    hybrid = definition(
        "Demo::Hybrid",
        specializations=[
            Specialization(kind="specializes", declared="Vehicle", target_id="Demo::Vehicle"),
            Specialization(kind="specializes", declared="Electric", target_id="Demo::Electric"),
        ],
    )
    source = render_module([hybrid, electric, vehicle])
    assert "class Hybrid(Electric):" in source
    assert "left out" not in source
    namespace: dict = {}
    exec(compile(source, "demo_types.py", "exec"), namespace)
    assert issubclass(namespace["Hybrid"], namespace["Electric"])
    assert issubclass(namespace["Hybrid"], namespace["Vehicle"])


def test_render_module_drops_a_base_python_cannot_linearize():
    """An order Python has no MRO for keeps the module importable and says what it left out."""
    left = definition("Demo::Left")
    right = definition("Demo::Right")
    # Opposite base orders have no common linearization, and neither implies the
    # other, so one edge cannot be expressed in Python at all.
    one = definition(
        "Demo::One",
        specializations=[
            Specialization(kind="specializes", declared="Left", target_id="Demo::Left"),
            Specialization(kind="specializes", declared="Right", target_id="Demo::Right"),
        ],
    )
    two = definition(
        "Demo::Two",
        specializations=[
            Specialization(kind="specializes", declared="Right", target_id="Demo::Right"),
            Specialization(kind="specializes", declared="Left", target_id="Demo::Left"),
        ],
    )
    both = definition(
        "Demo::Both",
        specializations=[
            Specialization(kind="specializes", declared="One", target_id="Demo::One"),
            Specialization(kind="specializes", declared="Two", target_id="Demo::Two"),
        ],
    )
    source = render_module([both, one, two, left, right])
    assert "class Both(One):" in source
    assert "# specializes Demo::Two, left out: Python cannot linearize it" in source
    namespace: dict = {}
    exec(compile(source, "demo_types.py", "exec"), namespace)
    assert namespace["Both"].__mro__[1] is namespace["One"]


def test_render_module_reports_a_base_left_implicit_by_a_base_that_is_then_dropped():
    """A base only a dropped base implied is reported, not silently lost."""
    left = definition("Demo::Left")
    right = definition("Demo::Right")
    extra = definition("Demo::Extra")
    one = definition(
        "Demo::One",
        specializations=[
            Specialization(kind="specializes", declared="Left", target_id="Demo::Left"),
            Specialization(kind="specializes", declared="Right", target_id="Demo::Right"),
        ],
    )
    two = definition(
        "Demo::Two",
        specializations=[
            Specialization(kind="specializes", declared="Right", target_id="Demo::Right"),
            Specialization(kind="specializes", declared="Left", target_id="Demo::Left"),
            Specialization(kind="specializes", declared="Extra", target_id="Demo::Extra"),
        ],
    )
    # Extra is implied by Two, but Two has no MRO alongside One, so dropping Two
    # takes Extra with it.
    both = definition(
        "Demo::Both",
        specializations=[
            Specialization(kind="specializes", declared="One", target_id="Demo::One"),
            Specialization(kind="subsets", declared="Extra", target_id="Demo::Extra"),
            Specialization(kind="specializes", declared="Two", target_id="Demo::Two"),
        ],
    )
    source = render_module([both, one, two, left, right, extra])
    assert "class Both(One):" in source
    assert "# subsets Demo::Extra, left out" in source
    assert "# specializes Demo::Two, left out" in source
    namespace: dict = {}
    exec(compile(source, "demo_types.py", "exec"), namespace)
    assert not issubclass(namespace["Both"], namespace["Extra"])


def test_render_module_reports_a_base_in_a_generalization_cycle():
    """A cyclic edge no base order can express is named in a comment."""
    first = definition(
        "Demo::First",
        specializations=[
            Specialization(kind="specializes", declared="Second", target_id="Demo::Second")
        ],
    )
    second = definition(
        "Demo::Second",
        specializations=[
            Specialization(kind="specializes", declared="First", target_id="Demo::First")
        ],
    )
    source = render_module([first, second])
    assert "left out" in source
    namespace: dict = {}
    exec(compile(source, "demo_types.py", "exec"), namespace)
    assert issubclass(namespace["First"], namespace["Second"]) != issubclass(
        namespace["Second"], namespace["First"]
    )


def test_collect_definitions_takes_type_and_multiplicity_from_a_redefinition():
    """A redefining feature that restates neither type nor multiplicity inherits both."""
    root = FakeSymbol(
        SymbolFacts(id="Demo", name="Demo", kind="package"),
        [
            FakeSymbol(SymbolFacts(id="Demo::Engine", name="Engine", kind="partDef")),
            FakeSymbol(
                SymbolFacts(id="Demo::Base", name="Base", kind="partDef"),
                [
                    FakeSymbol(
                        SymbolFacts(
                            id="Demo::Base::engine",
                            name="engine",
                            kind="partUsage",
                            type=TypeFacts(declared="Engine", resolved_id="Demo::Engine"),
                            multiplicity=Multiplicity("0", "*"),
                        )
                    )
                ],
            ),
            FakeSymbol(
                SymbolFacts(
                    id="Demo::Car",
                    name="Car",
                    kind="partDef",
                    specializations=(
                        Specialization(kind="specializes", declared="Base", target_id="Demo::Base"),
                    ),
                ),
                [
                    FakeSymbol(
                        SymbolFacts(
                            id="Demo::Car::engine",
                            name="engine",
                            kind="partUsage",
                            specializations=(
                                Specialization(
                                    kind="redefines",
                                    declared="engine",
                                    target_id="Demo::Base::engine",
                                ),
                            ),
                        )
                    )
                ],
            ),
        ],
    )

    definitions = {d.id: d for d in collect_definitions(root)}
    redefining = definitions["Demo::Car"].features[0]
    assert redefining.facts.type.resolved_id == "Demo::Engine"
    assert redefining.facts.multiplicity == Multiplicity("0", "*")

    source = render_module(list(definitions.values()))
    assert '_t.list_feature_value(self, "engine", _t.as_typed(Engine))' in source


def test_render_module_notes_ungenerated_base():
    """A specialization outside the model is reported in a comment, not silently dropped."""
    car = definition(
        "Demo::Car",
        specializations=[Specialization(kind="specializes", declared="Base", target_id="Lib::Base")],
    )
    source = render_module([car])
    assert "class Car(_t.TypedObject):" in source
    assert "# specializes Lib::Base, which has no generated class" in source


def test_render_module_uses_multiplicity_accessors():
    """List and optional features use the matching runtime accessor."""
    source = render_module(
        [
            definition(
                "Demo::Vehicle",
                features=[
                    feature("spare", type_facts=TypeFacts(primitive="Real"), multiplicity=Multiplicity("0", "1")),
                    feature("wheels", type_facts=TypeFacts(primitive="Real"), multiplicity=Multiplicity("0", "*")),
                ],
            )
        ]
    )
    assert '_t.optional_feature_value(self, "spare", _t.as_float)' in source
    assert '_t.list_feature_value(self, "wheels", _t.as_float)' in source


def test_render_module_empty_model():
    """A model with no definitions still emits an importable module."""
    source = render_module([])
    namespace: dict = {}
    exec(compile(source, "empty_types.py", "exec"), namespace)
    assert "TypedObject" not in namespace
