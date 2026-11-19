package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * A coordinate transformation held as a value: the frames it relates, the shape the model states
 * it in, and the pose that shape resolves to.
 *
 * @param name the transformation as a reader names it
 * @param transformationId FQN of the usage declaring it, absent for none
 * @param typeId FQN of the transformation's type, absent for none
 * @param instanceId the object the transformation was read from, absent for none
 * @param source the frame the transformation is stated in
 * @param target the frame it places, absent on a frame's own transformation
 * @param shape the shape the model states it in
 * @param pose where the target sits in the source, absent when no source is stated or the shape
 *     resolves to none
 */
public record CoordinateTransformation(
    String name,
    Optional<String> transformationId,
    Optional<String> typeId,
    Optional<Long> instanceId,
    Optional<CoordinateFrame> source,
    Optional<CoordinateFrame> target,
    Optional<Shape> shape,
    Optional<Pose> pose) {
  /**
   * Creates a coordinate transformation.
   *
   * @param name the name, never {@code null}
   * @param transformationId the declaring usage, never {@code null}
   * @param typeId the type, never {@code null}
   * @param instanceId the object read from, never {@code null}
   * @param source the source frame, never {@code null}
   * @param target the target frame, never {@code null}
   * @param shape the shape, never {@code null}
   * @param pose the pose, never {@code null}
   */
  public CoordinateTransformation {
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(transformationId, "transformationId");
    Objects.requireNonNull(typeId, "typeId");
    Objects.requireNonNull(instanceId, "instanceId");
    Objects.requireNonNull(source, "source");
    Objects.requireNonNull(target, "target");
    Objects.requireNonNull(shape, "shape");
    Objects.requireNonNull(pose, "pose");
  }

  /** The shape a model states a transformation in. */
  public sealed interface Shape {}

  /**
   * A {@code CoordinateFramePlacement}: the target's origin in the source and, when stated, its
   * basis directions there.
   *
   * @param origin the target's origin in the source
   * @param basisDirections the target's basis directions in the source, one per axis
   */
  public record Placement(Value origin, List<Value> basisDirections) implements Shape {
    /**
     * Creates a placement.
     *
     * @param origin the origin, never {@code null}
     * @param basisDirections the basis directions, never {@code null}
     */
    public Placement {
      Objects.requireNonNull(origin, "origin");
      basisDirections = List.copyOf(basisDirections);
    }
  }

  /**
   * A {@code TranslationRotationSequence}.
   *
   * @param steps the steps, in the order applied
   */
  public record Sequence(List<Step> steps) implements Shape {
    /**
     * Creates a sequence.
     *
     * @param steps the steps, never {@code null}
     */
    public Sequence {
      steps = List.copyOf(steps);
    }
  }

  /**
   * An {@code AffineTransformationMatrix3d}.
   *
   * @param rotation the 3×3 rotation matrix, row-major
   * @param translation the translation
   */
  public record Affine(List<Double> rotation, List<Double> translation) implements Shape {
    /**
     * Creates an affine transformation.
     *
     * @param rotation the rotation, never {@code null}
     * @param translation the translation, never {@code null}
     */
    public Affine {
      rotation = List.copyOf(rotation);
      translation = List.copyOf(translation);
    }
  }

  /** One step of a {@code TranslationRotationSequence}. */
  public sealed interface Step {
    /**
     * The Translation or Rotation object the step was read from.
     *
     * @return the object's id, absent for none
     */
    Optional<Long> instanceId();
  }

  /**
   * A shift by a vector.
   *
   * @param translation the vector shifted by
   * @param instanceId the object the step was read from
   */
  public record Translation(Value translation, Optional<Long> instanceId) implements Step {
    /**
     * Creates a translation.
     *
     * @param translation the vector, never {@code null}
     * @param instanceId the object read from, never {@code null}
     */
    public Translation {
      Objects.requireNonNull(translation, "translation");
      Objects.requireNonNull(instanceId, "instanceId");
    }
  }

  /**
   * A turn by an angle about an axis.
   *
   * @param axisDirection the axis turned about
   * @param angle the angle turned by
   * @param intrinsic about the axes as already rotated, rather than the source's fixed axes
   * @param instanceId the object the step was read from
   */
  public record Rotation(
      Value axisDirection, Quantity angle, boolean intrinsic, Optional<Long> instanceId)
      implements Step {
    /**
     * Creates a rotation.
     *
     * @param axisDirection the axis, never {@code null}
     * @param angle the angle, never {@code null}
     * @param intrinsic whether the axes turn with the rotation
     * @param instanceId the object read from, never {@code null}
     */
    public Rotation {
      Objects.requireNonNull(axisDirection, "axisDirection");
      Objects.requireNonNull(angle, "angle");
      Objects.requireNonNull(instanceId, "instanceId");
    }
  }

  /**
   * Where a target frame sits in its source frame, in the source's axis units: target coordinates
   * {@code v} lie at {@code origin + basis · v}.
   *
   * @param origin the target's origin, one number per source axis
   * @param basis the target's axes in the source, n×n row-major
   */
  public record Pose(List<Double> origin, List<Double> basis) {
    /**
     * Creates a pose.
     *
     * @param origin the origin, never {@code null}
     * @param basis the basis, never {@code null}
     */
    public Pose {
      origin = List.copyOf(origin);
      basis = List.copyOf(basis);
    }
  }
}
