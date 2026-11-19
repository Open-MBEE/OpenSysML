//! Coordinate frames and transformations held as values.

use crate::domain::{
    measurement_ref_from_wire, quantity_from_wire, value_from_wire, MeasurementRef, Quantity, Value,
};
use crate::error::Error;
use crate::wire;

/// A coordinate frame or measurement scale held as a value, sent by a service
/// advertising `coordinate_frame_values`; an answer, never an argument.
#[derive(Clone, Debug)]
pub struct CoordinateFrame {
    /// The frame as a reader names it: its declared name, else the expression
    /// that composed it (`datum / s`).
    pub name: String,
    /// FQN of the usage declaring the frame; `None` for one composed.
    pub frame_id: Option<String>,
    /// FQN of the frame's type; `None` for none.
    pub type_id: Option<String>,
    /// The object the frame was read from.
    pub instance_id: Option<i64>,
    /// The frame's `dimensions`.
    pub dimensions: Vec<i64>,
    /// The frame's `mRefs`, one per axis.
    pub axes: Vec<MeasurementRef>,
    /// What places the frame in another; its `target` is `None`, this frame
    /// being that target.
    pub transformation: Option<Box<CoordinateTransformation>>,
    /// Set for a measurement scale, which has no axes.
    pub scale: Option<MeasurementScale>,
}

/// The unit a measurement scale is measured in and, when the model defines the
/// scale against another reference, the point defining it.
#[derive(Clone, Debug, PartialEq)]
pub struct MeasurementScale {
    /// The unit the scale is measured in.
    pub unit: MeasurementRef,
    /// The defining point, when the model states one.
    pub mapping: Option<QuantityValueMapping>,
}

/// One point on a scale and the same point on the reference it is defined against.
#[derive(Clone, Debug, PartialEq)]
pub struct QuantityValueMapping {
    /// The point on the scale.
    pub mapped: Quantity,
    /// The same point on the reference.
    pub reference: Quantity,
}

/// A coordinate transformation held as a value: the frames it relates, the
/// shape the model states it in, and the pose that shape resolves to.
#[derive(Clone, Debug)]
pub struct CoordinateTransformation {
    /// The transformation as a reader names it.
    pub name: String,
    /// FQN of the usage declaring it; `None` for none.
    pub transformation_id: Option<String>,
    /// FQN of the transformation's type; `None` for none.
    pub type_id: Option<String>,
    /// The object the transformation was read from.
    pub instance_id: Option<i64>,
    /// The frame the transformation is stated in.
    pub source: Option<Box<CoordinateFrame>>,
    /// The frame it places; `None` on a frame's own `transformation`.
    pub target: Option<Box<CoordinateFrame>>,
    /// The shape the model states it in.
    pub shape: Option<FrameShape>,
    /// Where the target sits in the source; `None` when no source is stated
    /// or the shape resolves to none.
    pub pose: Option<FramePose>,
}

/// The shape a model states a transformation in.
#[derive(Clone, Debug, PartialEq)]
pub enum FrameShape {
    /// A `CoordinateFramePlacement`: the target's origin in the source and,
    /// when stated, its basis directions there.
    Placement {
        /// The target's origin in the source.
        origin: Box<Value>,
        /// The target's basis directions in the source, one per axis.
        basis_directions: Vec<Value>,
    },
    /// A `TranslationRotationSequence`: its steps in the order applied.
    Sequence(Vec<FrameStep>),
    /// An `AffineTransformationMatrix3d`: a 3×3 rotation, row-major, and a translation.
    Affine {
        /// The rotation matrix, row-major.
        rotation: Vec<f64>,
        /// The translation.
        translation: Vec<f64>,
    },
}

/// One step of a `TranslationRotationSequence`.
#[derive(Clone, Debug)]
pub struct FrameStep {
    /// What the step does.
    pub kind: FrameStepKind,
    /// The Translation or Rotation object the step was read from.
    pub instance_id: Option<i64>,
}

/// What a step of a `TranslationRotationSequence` does.
#[derive(Clone, Debug, PartialEq)]
pub enum FrameStepKind {
    /// A shift by a vector.
    Translation(Value),
    /// A turn by `angle` about `axis_direction`.
    Rotation {
        /// The axis turned about.
        axis_direction: Value,
        /// The angle turned by.
        angle: Quantity,
        /// About the axes as already rotated; otherwise the source's fixed axes.
        intrinsic: bool,
    },
}

/// Where a target frame sits in its source frame, in the source's axis units:
/// target coordinates `v` lie at `origin + basis · v`, `basis` n×n row-major.
#[derive(Clone, Debug, PartialEq)]
pub struct FramePose {
    /// The target's origin, one number per source axis.
    pub origin: Vec<f64>,
    /// The target's axes in the source, row-major.
    pub basis: Vec<f64>,
}

/// A frame is the object and usage it was read from when either has an object,
/// otherwise its dimensions, axes, scale and transformation, whatever names it.
impl PartialEq for CoordinateFrame {
    fn eq(&self, other: &Self) -> bool {
        if self.instance_id.is_some() || other.instance_id.is_some() {
            return self.instance_id == other.instance_id && self.frame_id == other.frame_id;
        }
        self.dimensions == other.dimensions
            && self.axes == other.axes
            && self.scale == other.scale
            && self.transformation == other.transformation
    }
}

/// One object read twice is one transformation; otherwise source, target and stated
/// shape decide, and a transformation of no library shape is only its object.
impl PartialEq for CoordinateTransformation {
    fn eq(&self, other: &Self) -> bool {
        let same_object = self.instance_id == other.instance_id
            && self.transformation_id == other.transformation_id;
        if same_object && self.instance_id.is_some() {
            return true;
        }
        if self.source != other.source || self.target != other.target {
            return false;
        }
        match (&self.shape, &other.shape) {
            (None, None) => same_object,
            (a, b) => a == b,
        }
    }
}

/// Steps are equal for one kind with equal parts, whatever object each was read from.
impl PartialEq for FrameStep {
    fn eq(&self, other: &Self) -> bool {
        self.kind == other.kind
    }
}

fn named(text: String) -> Option<String> {
    (!text.is_empty()).then_some(text)
}

pub(crate) fn frame_from_wire(v: wire::CoordinateFrame) -> Result<CoordinateFrame, Error> {
    let scale = match v.scale {
        Some(scale) => {
            let unit = scale
                .unit
                .ok_or_else(|| Error::Decode("a measurement scale names no unit".to_owned()))?;
            let mapping = match scale.mapping {
                Some(wire::QuantityValueMapping {
                    mapped: Some(mapped),
                    reference: Some(reference),
                }) => Some(QuantityValueMapping {
                    mapped: quantity_from_wire(mapped)?,
                    reference: quantity_from_wire(reference)?,
                }),
                Some(_) => {
                    return Err(Error::Decode(
                        "a quantity value mapping lacks one of its two points".to_owned(),
                    ))
                }
                None => None,
            };
            Some(MeasurementScale {
                unit: measurement_ref_from_wire(unit)?,
                mapping,
            })
        }
        None => None,
    };
    Ok(CoordinateFrame {
        name: v.name,
        frame_id: named(v.frame_id),
        type_id: named(v.type_id),
        instance_id: (v.instance_id != 0).then_some(v.instance_id),
        dimensions: v.dimensions,
        axes: v
            .axes
            .into_iter()
            .map(measurement_ref_from_wire)
            .collect::<Result<_, _>>()?,
        transformation: v
            .transformation
            .map(|t| transformation_from_wire(*t).map(Box::new))
            .transpose()?,
        scale,
    })
}

pub(crate) fn transformation_from_wire(
    v: wire::CoordinateTransformation,
) -> Result<CoordinateTransformation, Error> {
    use wire::coordinate_transformation::Shape;
    let endpoint = |frame: Option<Box<wire::CoordinateFrame>>| {
        frame.map(|f| frame_from_wire(*f).map(Box::new)).transpose()
    };
    let shape = match v.shape {
        Some(Shape::Placement(p)) => {
            let origin = p
                .origin
                .ok_or_else(|| Error::Decode("a frame placement states no origin".to_owned()))?;
            Some(FrameShape::Placement {
                origin: Box::new(value_from_wire(*origin)?),
                basis_directions: p
                    .basis_directions
                    .into_iter()
                    .map(value_from_wire)
                    .collect::<Result<_, _>>()?,
            })
        }
        Some(Shape::Sequence(s)) => Some(FrameShape::Sequence(
            s.steps
                .into_iter()
                .map(step_from_wire)
                .collect::<Result<_, _>>()?,
        )),
        Some(Shape::Affine(a)) => Some(FrameShape::Affine {
            rotation: a.rotation,
            translation: a.translation,
        }),
        None => None,
    };
    Ok(CoordinateTransformation {
        name: v.name,
        transformation_id: named(v.transformation_id),
        type_id: named(v.type_id),
        instance_id: (v.instance_id != 0).then_some(v.instance_id),
        source: endpoint(v.source)?,
        target: endpoint(v.target)?,
        shape,
        pose: v.pose.map(|p| FramePose {
            origin: p.origin,
            basis: p.basis,
        }),
    })
}

fn step_from_wire(step: wire::FrameStep) -> Result<FrameStep, Error> {
    use wire::frame_step::Step;
    let kind = match step.step {
        Some(Step::Translation(v)) => FrameStepKind::Translation(value_from_wire(v)?),
        Some(Step::Rotation(r)) => {
            let (Some(axis), Some(angle)) = (r.axis_direction, r.angle) else {
                return Err(Error::Decode(
                    "a frame rotation lacks its axis or its angle".to_owned(),
                ));
            };
            FrameStepKind::Rotation {
                axis_direction: value_from_wire(axis)?,
                angle: quantity_from_wire(angle)?,
                intrinsic: r.is_intrinsic,
            }
        }
        None => {
            return Err(Error::Decode(
                "a transformation step is neither a translation nor a rotation".to_owned(),
            ))
        }
    };
    Ok(FrameStep {
        kind,
        instance_id: (step.instance_id != 0).then_some(step.instance_id),
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::domain::Magnitude;

    fn millimetre() -> wire::MeasurementRef {
        wire::MeasurementRef {
            unit: "mm".to_owned(),
            unit_id: "SI::millimetre".to_owned(),
            unit_term: Some(wire::UnitTerm {
                scale_num: 1.0,
                scale_den: 1000.0,
                factors: vec![wire::UnitFactor {
                    unit_id: "SI::metre".to_owned(),
                    exponent: 1.0,
                }],
            }),
        }
    }

    fn frame(name: &str) -> wire::CoordinateFrame {
        wire::CoordinateFrame {
            name: name.to_owned(),
            frame_id: format!("Frames::{name}"),
            type_id: "ISQSpaceTime::CartesianSpatial3dCoordinateFrame".to_owned(),
            dimensions: vec![3],
            axes: vec![millimetre(); 3],
            ..Default::default()
        }
    }

    fn length(real: f64) -> wire::Quantity {
        let unit = millimetre();
        wire::Quantity {
            unit: unit.unit,
            unit_term: unit.unit_term,
            magnitude: Some(wire::quantity::Magnitude::RealMagnitude(real)),
            ..Default::default()
        }
    }

    fn vector(reals: [f64; 3]) -> wire::Value {
        wire::Value {
            kind: Some(wire::value::Kind::VectorQuantity(wire::VectorQuantity {
                components: reals.into_iter().map(length).collect(),
            })),
        }
    }

    fn decode(kind: wire::value::Kind) -> Result<Value, Error> {
        value_from_wire(wire::Value { kind: Some(kind) })
    }

    #[test]
    fn a_frame_keeps_its_identity_and_axes() {
        let Value::CoordinateFrame(got) =
            decode(wire::value::Kind::CoordinateFrame(Box::new(frame("datum"))))
                .expect("a frame decodes")
        else {
            panic!("want a coordinate frame");
        };
        assert_eq!(got.name, "datum");
        assert_eq!(got.frame_id.as_deref(), Some("Frames::datum"));
        assert_eq!(got.instance_id, None);
        assert_eq!(got.dimensions, vec![3]);
        assert_eq!(got.axes.len(), 3);
        assert_eq!(got.axes[0].unit_id.as_deref(), Some("SI::millimetre"));
        assert!(got.transformation.is_none() && got.scale.is_none());
    }

    #[test]
    fn a_posed_frame_carries_its_sequence_source_and_pose() {
        let mut posed = frame("lbcf");
        posed.transformation = Some(Box::new(wire::CoordinateTransformation {
            name: "transformation".to_owned(),
            source: Some(Box::new(frame("datum"))),
            shape: Some(wire::coordinate_transformation::Shape::Sequence(
                wire::TranslationRotationSequence {
                    steps: vec![
                        wire::FrameStep {
                            instance_id: 7,
                            step: Some(wire::frame_step::Step::Translation(vector([
                                10.0, 0.0, 0.0,
                            ]))),
                        },
                        wire::FrameStep {
                            instance_id: 8,
                            step: Some(wire::frame_step::Step::Rotation(wire::FrameRotation {
                                axis_direction: Some(vector([0.0, 0.0, 1.0])),
                                angle: Some(length(180.0)),
                                is_intrinsic: true,
                            })),
                        },
                    ],
                },
            )),
            pose: Some(wire::FramePose {
                origin: vec![10.0, 0.0, 0.0],
                basis: vec![-1.0, 0.0, 0.0, 0.0, -1.0, 0.0, 0.0, 0.0, 1.0],
            }),
            ..Default::default()
        }));
        let Value::CoordinateFrame(got) =
            decode(wire::value::Kind::CoordinateFrame(Box::new(posed))).expect("a frame decodes")
        else {
            panic!("want a coordinate frame");
        };
        let t = got
            .transformation
            .expect("the frame carries its transformation");
        assert_eq!(
            t.source.as_ref().and_then(|s| s.frame_id.as_deref()),
            Some("Frames::datum")
        );
        assert!(t.target.is_none());
        let Some(FrameShape::Sequence(steps)) = &t.shape else {
            panic!("want a sequence, got {:?}", t.shape);
        };
        assert!(matches!(
            &steps[0],
            FrameStep {
                kind: FrameStepKind::Translation(Value::VectorQuantity(_)),
                instance_id: Some(7)
            }
        ));
        let FrameStepKind::Rotation {
            angle, intrinsic, ..
        } = &steps[1].kind
        else {
            panic!("want a rotation, got {:?}", steps[1]);
        };
        assert!(*intrinsic);
        assert_eq!(angle.magnitude, Magnitude::Real(180.0));
        assert_eq!(t.pose.expect("a pose").origin, vec![10.0, 0.0, 0.0]);
    }

    #[test]
    fn a_transformation_keeps_its_endpoints_and_shape() {
        let placed = wire::CoordinateTransformation {
            name: "placement".to_owned(),
            source: Some(Box::new(frame("datum"))),
            target: Some(Box::new(frame("placed"))),
            shape: Some(wire::coordinate_transformation::Shape::Placement(Box::new(
                wire::FramePlacement {
                    origin: Some(Box::new(vector([1.0, 2.0, 3.0]))),
                    basis_directions: vec![vector([1.0, 0.0, 0.0])],
                },
            ))),
            ..Default::default()
        };
        let Value::CoordinateTransformation(got) = decode(
            wire::value::Kind::CoordinateTransformation(Box::new(placed)),
        )
        .expect("a transformation decodes") else {
            panic!("want a coordinate transformation");
        };
        assert_eq!(got.target.as_ref().expect("a target").name, "placed");
        assert!(got.pose.is_none());
        assert!(matches!(
            got.shape,
            Some(FrameShape::Placement { ref basis_directions, .. }) if basis_directions.len() == 1
        ));

        let affine = wire::CoordinateTransformation {
            shape: Some(wire::coordinate_transformation::Shape::Affine(
                wire::AffineTransformation3d {
                    rotation: vec![1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0],
                    translation: vec![1.0, 2.0, 3.0],
                },
            )),
            ..Default::default()
        };
        let got = decode(wire::value::Kind::CoordinateTransformation(Box::new(
            affine,
        )))
        .expect("an affine transformation decodes");
        assert!(matches!(
            got,
            Value::CoordinateTransformation(t)
                if matches!(&t.shape, Some(FrameShape::Affine { translation, .. }) if translation == &[1.0, 2.0, 3.0])
        ));
    }

    #[test]
    fn a_measurement_scale_keeps_its_unit_and_mapping() {
        let scale = wire::CoordinateFrame {
            name: "celsius".to_owned(),
            scale: Some(wire::MeasurementScale {
                unit: Some(millimetre()),
                mapping: Some(wire::QuantityValueMapping {
                    mapped: Some(length(0.0)),
                    reference: Some(length(273.15)),
                }),
            }),
            ..Default::default()
        };
        let Value::CoordinateFrame(got) =
            decode(wire::value::Kind::CoordinateFrame(Box::new(scale))).expect("a scale decodes")
        else {
            panic!("want a coordinate frame");
        };
        let mapping = got.scale.expect("a scale").mapping.expect("a mapping");
        assert_eq!(mapping.reference.magnitude, Magnitude::Real(273.15));
    }

    #[test]
    fn a_frame_that_contradicts_itself_does_not_decode() {
        let no_unit = wire::CoordinateFrame {
            axes: vec![wire::MeasurementRef::default()],
            ..Default::default()
        };
        assert!(decode(wire::value::Kind::CoordinateFrame(Box::new(no_unit))).is_err());
        let no_step = wire::CoordinateTransformation {
            shape: Some(wire::coordinate_transformation::Shape::Sequence(
                wire::TranslationRotationSequence {
                    steps: vec![wire::FrameStep::default()],
                },
            )),
            ..Default::default()
        };
        assert!(decode(wire::value::Kind::CoordinateTransformation(Box::new(
            no_step
        )))
        .is_err());
    }

    #[test]
    fn frames_and_transformations_compare_as_the_service_does() {
        let read = |frame: wire::CoordinateFrame| {
            decode(wire::value::Kind::CoordinateFrame(Box::new(frame))).expect("a frame decodes")
        };
        let read_t = |t: wire::CoordinateTransformation| {
            decode(wire::value::Kind::CoordinateTransformation(Box::new(t)))
                .expect("a transformation decodes")
        };
        let object = |name: &str, instance_id: i64| wire::CoordinateFrame {
            instance_id,
            ..frame(name)
        };
        let placement = |x: f64| {
            Some(wire::coordinate_transformation::Shape::Placement(Box::new(
                wire::FramePlacement {
                    origin: Some(Box::new(vector([x, 0.0, 0.0]))),
                    ..Default::default()
                },
            )))
        };
        let own = wire::CoordinateTransformation {
            name: "placed".to_owned(),
            instance_id: 9,
            source: Some(Box::new(object("datum", 3))),
            shape: placement(1.0),
            pose: Some(wire::FramePose {
                origin: vec![1.0, 0.0, 0.0],
                basis: vec![1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0],
            }),
            ..Default::default()
        };
        let posed = |instance_id| wire::CoordinateFrame {
            transformation: Some(Box::new(own.clone())),
            ..object("posed", instance_id)
        };

        assert_eq!(read(posed(7)), read(object("posed", 7)));
        assert_ne!(read(posed(7)), read(posed(8)));
        assert_ne!(read(object("posed", 7)), read(object("datum", 7)));

        let composed = || wire::CoordinateFrame {
            frame_id: String::new(),
            ..frame("datum / s")
        };
        let renamed = wire::CoordinateFrame {
            type_id: "Frames::Rate".to_owned(),
            ..frame("rate")
        };
        assert_eq!(read(composed()), read(renamed));
        let shorter = wire::CoordinateFrame {
            dimensions: vec![1],
            ..composed()
        };
        assert_ne!(read(composed()), read(shorter));
        assert_ne!(read(composed()), read(object("datum", 3)));

        let direct = wire::CoordinateTransformation {
            name: "trs".to_owned(),
            target: Some(Box::new(object("posed", 7))),
            pose: None,
            ..own.clone()
        };
        assert_eq!(read_t(own.clone()), read_t(direct));
        let constructed = |name: &str, x: f64| wire::CoordinateTransformation {
            name: name.to_owned(),
            source: Some(Box::new(object("datum", 3))),
            shape: placement(x),
            ..Default::default()
        };
        let with_pose = wire::CoordinateTransformation {
            pose: own.pose.clone(),
            ..constructed("b", 1.0)
        };
        assert_eq!(read_t(constructed("a", 1.0)), read_t(with_pose));
        assert_ne!(read_t(constructed("a", 1.0)), read_t(constructed("a", 2.0)));
        let shapeless = |instance_id| wire::CoordinateTransformation {
            instance_id,
            shape: None,
            ..constructed("a", 1.0)
        };
        assert_ne!(read_t(shapeless(4)), read_t(shapeless(5)));
    }
}
