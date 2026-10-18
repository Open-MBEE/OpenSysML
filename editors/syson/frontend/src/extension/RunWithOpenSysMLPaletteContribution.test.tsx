import { MockedProvider } from '@apollo/client/testing';
import { TreePaletteContext, type TreePaletteContextValue } from '@eclipse-sirius/sirius-components-trees';
import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { RunWithOpenSysMLPaletteContribution } from './RunWithOpenSysMLPaletteContribution';

const tool = {
  id: 'runWithOpenSysML',
  label: 'Run with OpenSysML…',
  iconURL: [],
  __typename: 'SingleClickTreeItemTool',
};

const context: TreePaletteContextValue = {
  editingContextId: 'ctx',
  treeId: 'explorer://tree',
  expanded: [],
  item: {
    id: 'obj',
    label: { styledStringFragments: [{ text: 'Car' }] },
    kind: 'kind',
    iconURL: [''],
    hasChildren: false,
    children: [],
    expanded: false,
    editable: false,
    deletable: false,
    selectable: true,
  },
  readOnly: false,
  selectedTreeItems: [],
  selectTreeItems: vi.fn(),
  onExpandedElementChange: vi.fn(),
  onDirectEditClick: vi.fn(),
  expandItem: vi.fn(),
  onClose: vi.fn(),
};

const renderTool = (value: TreePaletteContextValue, searchedValue: string | null = null, onInvoked = vi.fn()) =>
  render(
    <MockedProvider>
      <TreePaletteContext.Provider value={value}>
        <RunWithOpenSysMLPaletteContribution
          tool={tool}
          representationElementIds={['obj']}
          onInvoked={onInvoked}
          asLastToolUsed={false}
          isToolInPalette={true}
          searchedValue={searchedValue}
        />
      </TreePaletteContext.Provider>
    </MockedProvider>
  );

describe('RunWithOpenSysMLPaletteContribution', () => {
  it('renders nothing outside the explorer or when read-only', () => {
    expect(renderTool({ ...context, treeId: 'diagram://d' }).container).toBeEmptyDOMElement();
    expect(renderTool({ ...context, readOnly: true }).container).toBeEmptyDOMElement();
  });

  it('hides itself from a search it does not match', () => {
    expect(renderTool(context, 'rename').container).toBeEmptyDOMElement();
    renderTool(context, 'opensysml');
    expect(screen.getByTestId('run-with-opensysml-tool')).toBeInTheDocument();
  });

  it('tells the palette it was invoked and opens the dialog', () => {
    const onInvoked = vi.fn();
    renderTool(context, null, onInvoked);
    fireEvent.click(screen.getByTestId('run-with-opensysml-tool'));
    expect(onInvoked).toHaveBeenCalledTimes(1);
    expect(screen.getByText('Run with OpenSysML: Car')).toBeInTheDocument();
  });
});
