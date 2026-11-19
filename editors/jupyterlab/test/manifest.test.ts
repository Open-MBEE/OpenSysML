import assert from 'node:assert/strict';
import { test } from 'node:test';

import manifest from '../package.json';

test('the bundle asks the host for each runtime dependency at the range package.json declares', () => {
  // Without an entry the builder stamps the build's own @jupyterlab/core-meta version as the requirement.
  const shared: Record<string, unknown> = manifest.jupyterlab.sharedPackages;
  const declared = Object.fromEntries(
    Object.entries(manifest.dependencies).map(([name, range]) => [name, { bundled: false, singleton: true, requiredVersion: range }])
  );
  assert.deepEqual(shared, declared);
});
