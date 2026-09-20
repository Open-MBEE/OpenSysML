import { ExtensionRegistry } from '@eclipse-sirius/sirius-components-core';
import { addOpenSysMLContributions } from './opensysmlExtensionRegistry';
import { treeItemContextMenuEntryOverrideExtensionPoint } from '@eclipse-sirius/sirius-components-trees';
import { describe, expect, it } from 'vitest';

describe('opensysmlExtensionRegistry', () => {
  it('keeps existing contributions and appends the OpenSysML contribution', () => {
    const registry = new ExtensionRegistry();
    const existing = { canHandle: () => true, component: (() => null) as never };
    registry.putData(treeItemContextMenuEntryOverrideExtensionPoint, { identifier: 'existing', data: [existing] });
    addOpenSysMLContributions(registry);
    const data = registry.getData(treeItemContextMenuEntryOverrideExtensionPoint);
    expect(data?.identifier).toBe('existing');
    expect(data?.data).toHaveLength(2);
    expect(data?.data[1].canHandle({ id: 'runWithOpenSysML' } as never)).toBe(true);
    expect(data?.data[1].canHandle({ id: 'other' } as never)).toBe(false);
  });

  it('puts the OpenSysML contribution into an empty registry', () => {
    const registry = new ExtensionRegistry();
    addOpenSysMLContributions(registry);
    expect(registry.getData(treeItemContextMenuEntryOverrideExtensionPoint)?.data).toHaveLength(1);
  });
});
