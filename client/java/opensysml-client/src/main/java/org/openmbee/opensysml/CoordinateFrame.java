package org.openmbee.opensysml;

import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * A coordinate frame or measurement scale held as a value: what a feature typed by a {@code
 * MeasurementReferences::CoordinateFrame} evaluates to.
 *
 * @param name the frame as a reader names it: its declared name, else the expression that
 *     composed it ({@code datum / s})
 * @param frameId FQN of the usage declaring the frame, absent for one composed
 * @param typeId FQN of the frame's type, absent for none
 * @param instanceId the object the frame was read from, absent for none
 * @param dimensions the frame's {@code dimensions}
 * @param axes the frame's {@code mRefs}, one per axis
 * @param transformation what places the frame in another, its target absent because this frame
 *     is that target
 * @param scale set for a measurement scale, which has no axes
 */
public record CoordinateFrame(
    String name,
    Optional<String> frameId,
    Optional<String> typeId,
    Optional<Long> instanceId,
    List<Long> dimensions,
    List<Value.MeasurementRefValue> axes,
    Optional<CoordinateTransformation> transformation,
    Optional<Scale> scale) {
  /**
   * Creates a coordinate frame.
   *
   * @param name the frame's name, never {@code null}
   * @param frameId the declaring usage, never {@code null}
   * @param typeId the frame's type, never {@code null}
   * @param instanceId the object read from, never {@code null}
   * @param dimensions the dimensions, never {@code null}
   * @param axes the axes, never {@code null}
   * @param transformation the transformation, never {@code null}
   * @param scale the scale, never {@code null}
   */
  public CoordinateFrame {
    Objects.requireNonNull(name, "name");
    Objects.requireNonNull(frameId, "frameId");
    Objects.requireNonNull(typeId, "typeId");
    Objects.requireNonNull(instanceId, "instanceId");
    dimensions = List.copyOf(dimensions);
    axes = List.copyOf(axes);
    Objects.requireNonNull(transformation, "transformation");
    Objects.requireNonNull(scale, "scale");
  }

  // As the service compares: the object and usage read from when either frame has an object,
  // otherwise dimensions, axes, scale and transformation, whatever names the frame.
  @Override
  public boolean equals(Object other) {
    if (!(other instanceof CoordinateFrame that)) {
      return false;
    }
    if (instanceId.isPresent() || that.instanceId.isPresent()) {
      return instanceId.equals(that.instanceId) && frameId.equals(that.frameId);
    }
    return dimensions.equals(that.dimensions)
        && axes.equals(that.axes)
        && scale.equals(that.scale)
        && transformation.equals(that.transformation);
  }

  @Override
  public int hashCode() {
    return instanceId.isPresent()
        ? Objects.hash(instanceId, frameId)
        : Objects.hash(dimensions, axes.size());
  }

  /**
   * The unit a measurement scale is measured in and, when the model defines the scale against
   * another reference, the point defining it.
   *
   * @param unit the unit the scale is measured in
   * @param mapping the defining point, absent when the model states none
   */
  public record Scale(Value.MeasurementRefValue unit, Optional<Mapping> mapping) {
    /**
     * Creates a scale.
     *
     * @param unit the unit, never {@code null}
     * @param mapping the defining point, never {@code null}
     */
    public Scale {
      Objects.requireNonNull(unit, "unit");
      Objects.requireNonNull(mapping, "mapping");
    }
  }

  /**
   * One point on a scale and the same point on the reference the scale is defined against.
   *
   * @param mapped the point on the scale
   * @param reference the same point on the reference
   */
  public record Mapping(Quantity mapped, Quantity reference) {
    /**
     * Creates a mapping.
     *
     * @param mapped the point on the scale, never {@code null}
     * @param reference the point on the reference, never {@code null}
     */
    public Mapping {
      Objects.requireNonNull(mapped, "mapped");
      Objects.requireNonNull(reference, "reference");
    }
  }
}
