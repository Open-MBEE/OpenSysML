import { ExtensionRegistry } from '@eclipse-sirius/sirius-components-core';
import { paletteToolOverrideExtensionPoint } from '@eclipse-sirius/sirius-components-palette';
import { treeItemContextMenuEntryOverrideExtensionPoint } from '@eclipse-sirius/sirius-components-trees';
import { describe, expect, it } from 'vitest';
import { addOpenSysMLContributions, opensysmlExtensionRegistry } from './opensysmlExtensionRegistry';

describe('opensysmlExtensionRegistry', () => {
  it('keeps existing contributions and appends the OpenSysML contributions', () => {
    const registry = new ExtensionRegistry();
    const existing = { canHandle: () => true, component: (() => null) as never };
    registry.putData(treeItemContextMenuEntryOverrideExtensionPoint, { identifier: 'existing', data: [existing] });
    registry.putData(paletteToolOverrideExtensionPoint, { identifier: 'existingPalette', data: [existing] });
    addOpenSysMLContributions(registry);

    const menu = registry.getData(treeItemContextMenuEntryOverrideExtensionPoint);
    expect(menu?.identifier).toBe('existing');
    expect(menu?.data).toHaveLength(2);
    expect(menu?.data[1].canHandle({ id: 'runWithOpenSysML' } as never)).toBe(true);
    expect(menu?.data[1].canHandle({ id: 'other' } as never)).toBe(false);

    const palette = registry.getData(paletteToolOverrideExtensionPoint);
    expect(palette?.identifier).toBe('existingPalette');
    expect(palette?.data).toHaveLength(2);
    expect(palette?.data[1].canHandle('tree', { id: 'runWithOpenSysML' } as never)).toBe(true);
    expect(palette?.data[1].canHandle('diagram', { id: 'runWithOpenSysML' } as never)).toBe(false);
    expect(palette?.data[1].canHandle('tree', { id: 'other' } as never)).toBe(false);
  });

  it('puts both OpenSysML contributions into an empty registry', () => {
    const registry = new ExtensionRegistry();
    addOpenSysMLContributions(registry);
    expect(registry.getData(treeItemContextMenuEntryOverrideExtensionPoint)?.data).toHaveLength(1);
    expect(registry.getData(paletteToolOverrideExtensionPoint)?.data).toHaveLength(1);
    expect(opensysmlExtensionRegistry.getData(paletteToolOverrideExtensionPoint)?.data).toHaveLength(1);
  });
});
