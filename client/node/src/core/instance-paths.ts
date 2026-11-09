/**
 * Every root-relative path that reaches one object, as the `InstancePaths` RPC
 * answers it: aliases through connector ends and reference usages included,
 * where `Instance.reachedBy` gives only the path the traversal came by.
 */
import type { InstancePathsResponse } from "../generated/sysml_pb.js";

/** One step of a path: a feature of the object before it, and the object the step reaches. */
export interface PathStep {
  /** The holder's feature, a key of its `features`. */
  featureName: string;
  /** The qualified name of the feature as declared. */
  featureSymbolId: string;
  /** The position among the values of a multi-valued feature, counted from 1 as an object path indexes them; 0 for a single-valued one. */
  index: number;
  /** The object the step reaches. */
  instanceId: bigint;
}

/** One root-relative path to an object. */
export interface InstancePath {
  /** The root the path starts from. */
  rootInstanceId: bigint;
  /** The qualified name the root was instantiated as; empty when the name no longer binds it and `#<id>` addresses it. */
  rootSymbolId: string;
  /** The steps as an object path (`wheels[2].hub`); empty when the object is the root. */
  path: string;
  readonly steps: readonly PathStep[];
}

/** What one `InstancePaths` call answered. */
export class InstancePaths {
  /**
   * The paths in traversal order: the roots the service holds in id order and,
   * from each, depth-first over the features of an object in name order and
   * the values of a feature in their order.
   */
  readonly paths: readonly InstancePath[];
  /** Whether the service bounded its search short of every path: more reach the object than it lists, or one deeper than it walks. */
  readonly truncated: boolean;

  constructor(paths: readonly InstancePath[], truncated: boolean) {
    this.paths = paths;
    this.truncated = truncated;
  }

  /** The number of paths listed. */
  get length(): number {
    return this.paths.length;
  }

  [Symbol.iterator](): Iterator<InstancePath> {
    return this.paths[Symbol.iterator]();
  }
}

/** Decodes an `InstancePathsResponse` the service answered without error. */
export function instancePathsOf(response: InstancePathsResponse): InstancePaths {
  return new InstancePaths(
    response.paths.map((path) => ({
      rootInstanceId: path.root?.instanceId ?? 0n,
      rootSymbolId: path.root?.symbolId ?? "",
      path: path.path,
      steps: path.steps.map((step) => ({
        featureName: step.featureName,
        featureSymbolId: step.featureSymbolId,
        index: step.index,
        instanceId: step.instanceId,
      })),
    })),
    response.truncated,
  );
}
