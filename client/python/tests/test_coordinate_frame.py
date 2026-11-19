"""Coordinate frames and transformations held as values.

The service sends a feature typed by a ``CoordinateFrame`` as the
``coordinate_frame`` arm of ``Value`` and a ``CoordinateTransformation`` as the
``coordinate_transformation`` arm, so the client reads each as the structure it
is rather than as an unsupported null.
"""

import opensysml
from opensysml.capabilities import CAPABILITY_COORDINATE_FRAME_VALUES
from opensysml.errors import UnsupportedValueError
from opensysml.proto import sysml_pb2
from opensysml.values import (
    AffineTransformation3d, CoordinateFrame, CoordinateTransformation, FramePose, Quantity,
    VectorQuantity, value_to_python,
)

import pytest


def millimetre():
    return sysml_pb2.MeasurementRef(
        unit="mm", unit_id="SI::millimetre",
        unit_term=sysml_pb2.UnitTerm(
            scale_num=1, scale_den=1000,
            factors=[sysml_pb2.UnitFactor(unit_id="SI::metre", exponent=1)]),
    )


def frame(name, **fields):
    return sysml_pb2.CoordinateFrame(
        name=name, frame_id="Frames::" + name,
        type_id="ISQSpaceTime::CartesianSpatial3dCoordinateFrame",
        dimensions=[3], axes=[millimetre()] * 3, **fields)


def length(magnitude):
    ref = millimetre()
    return sysml_pb2.Quantity(real_magnitude=magnitude, unit=ref.unit, unit_term=ref.unit_term)


def vector(*magnitudes):
    return sysml_pb2.Value(vector_quantity=sysml_pb2.VectorQuantity(
        components=[length(m) for m in magnitudes]))


def test_a_frame_reads_with_its_identity_and_axes():
    got = value_to_python(sysml_pb2.Value(coordinate_frame=frame("datum", instance_id=4)))
    assert isinstance(got, CoordinateFrame)
    assert (got.name, got.frame_id, got.instance_id) == ("datum", "Frames::datum", 4)
    assert got.type_id == "ISQSpaceTime::CartesianSpatial3dCoordinateFrame"
    assert got.dimensions == (3,)
    assert [axis.unit_id for axis in got.axes] == ["SI::millimetre"] * 3
    assert got.transformation is None and got.scale is None
    assert str(got) == "datum [mm, mm, mm]"


def test_a_posed_frame_carries_its_sequence_and_pose():
    rotation = sysml_pb2.FrameRotation(
        axis_direction=vector(0.0, 0.0, 1.0),
        angle=sysml_pb2.Quantity(int_magnitude=180, unit="°", unit_term=sysml_pb2.UnitTerm(scale_num=1, scale_den=1)),
        is_intrinsic=True)
    posed = frame("lbcf", transformation=sysml_pb2.CoordinateTransformation(
        name="transformation", source=frame("datum"),
        sequence=sysml_pb2.TranslationRotationSequence(steps=[
            sysml_pb2.FrameStep(translation=vector(10.0, 0.0, 0.0), instance_id=7),
            sysml_pb2.FrameStep(rotation=rotation, instance_id=8),
        ]),
        pose=sysml_pb2.FramePose(origin=[10, 0, 0], basis=[-1, 0, 0, 0, -1, 0, 0, 0, 1]),
    ))
    got = value_to_python(sysml_pb2.Value(coordinate_frame=posed)).transformation
    assert got.source.frame_id == "Frames::datum" and got.target is None
    assert got.placement is None and got.affine is None
    shift, turn = got.steps
    assert isinstance(shift.translation, VectorQuantity) and shift.rotation is None
    assert shift.instance_id == 7
    assert turn.translation is None and turn.rotation.intrinsic
    assert isinstance(turn.rotation.angle, Quantity) and turn.rotation.angle.magnitude == 180
    assert got.pose == FramePose(origin=(10, 0, 0), basis=(-1, 0, 0, 0, -1, 0, 0, 0, 1))


def test_a_transformation_reads_its_endpoints_and_shape():
    placed = sysml_pb2.CoordinateTransformation(
        name="placement", transformation_id="Frames::placed::transformation",
        source=frame("datum"), target=frame("placed"),
        placement=sysml_pb2.FramePlacement(
            origin=vector(1.0, 2.0, 3.0), basis_directions=[vector(1.0, 0.0, 0.0)]))
    got = value_to_python(sysml_pb2.Value(coordinate_transformation=placed))
    assert isinstance(got, CoordinateTransformation)
    assert (got.source.name, got.target.name) == ("datum", "placed")
    assert got.steps is None and got.pose is None
    assert [q.magnitude for q in got.placement.origin] == [1.0, 2.0, 3.0]
    assert len(got.placement.basis_directions) == 1
    assert str(got) == "placement (datum → placed)"

    affine = sysml_pb2.CoordinateTransformation(affine=sysml_pb2.AffineTransformation3d(
        rotation=[1, 0, 0, 0, 1, 0, 0, 0, 1], translation=[1, 2, 3]))
    got = value_to_python(sysml_pb2.Value(coordinate_transformation=affine))
    assert got.affine == AffineTransformation3d(rotation=(1, 0, 0, 0, 1, 0, 0, 0, 1), translation=(1, 2, 3))
    assert str(got) == " (? → ?)"


def test_a_measurement_scale_reads_its_unit_and_mapping():
    scale = sysml_pb2.CoordinateFrame(name="celsius", scale=sysml_pb2.MeasurementScale(
        unit=millimetre(),
        mapping=sysml_pb2.QuantityValueMapping(mapped=length(0.0), reference=length(273.15))))
    got = value_to_python(sysml_pb2.Value(coordinate_frame=scale))
    assert got.axes == () and got.scale.unit.unit_id == "SI::millimetre"
    assert (got.scale.mapped.magnitude, got.scale.reference.magnitude) == (0.0, 273.15)
    assert str(got) == "celsius [mm]"


def test_a_frame_with_an_unreadable_part_is_unsupported():
    broken = sysml_pb2.CoordinateFrame(name="f", axes=[sysml_pb2.MeasurementRef()])
    with pytest.raises(UnsupportedValueError):
        value_to_python(sysml_pb2.Value(coordinate_frame=broken))
    step = sysml_pb2.CoordinateTransformation(
        sequence=sysml_pb2.TranslationRotationSequence(steps=[sysml_pb2.FrameStep()]))
    with pytest.raises(UnsupportedValueError, match="step"):
        value_to_python(sysml_pb2.Value(coordinate_transformation=step))


def test_frames_in_a_sequence_are_read_element_by_element():
    sequence = sysml_pb2.Value(sequence=sysml_pb2.ValueSequence(elements=[
        sysml_pb2.Value(int_value=1),
        sysml_pb2.Value(coordinate_frame=frame("datum")),
    ]))
    first, second = value_to_python(sequence)
    assert first == 1 and second.name == "datum"


def test_frames_are_exported_with_their_capability():
    assert opensysml.CoordinateFrame is CoordinateFrame
    assert opensysml.CoordinateTransformation is CoordinateTransformation
    assert CAPABILITY_COORDINATE_FRAME_VALUES == "coordinate_frame_values"
