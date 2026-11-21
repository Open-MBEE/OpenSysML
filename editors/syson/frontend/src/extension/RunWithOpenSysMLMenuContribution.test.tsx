import { fireEvent, render, screen } from '@testing-library/react';
import { MockedProvider } from '@apollo/client/testing';
import { RunWithOpenSysMLMenuContribution } from './RunWithOpenSysMLMenuContribution';
import type { TreeItemContextMenuComponentProps } from '@eclipse-sirius/sirius-components-trees';
import { describe, expect, it, vi } from 'vitest';

const props = {
  editingContextId: 'ctx',
  treeId: 'explorer://tree',
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
  entry: null,
  readOnly: false,
  expandItem: vi.fn(),
  selectTreeItems: vi.fn(),
  onExpandedElementChange: vi.fn(),
  onClose: vi.fn(),
  key: 'key',
  expanded: [],
  maxDepth: 0,
  selectedTreeItemIds: [],
} as TreeItemContextMenuComponentProps;

describe('RunWithOpenSysMLMenuContribution', () => {
  it('renders nothing outside the explorer', () => {
    const { container } = render(<RunWithOpenSysMLMenuContribution {...props} treeId="diagram://diagram" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('opens the dialog from the explorer menu', () => {
    render(
      <MockedProvider>
        <RunWithOpenSysMLMenuContribution {...props} />
      </MockedProvider>
    );
    fireEvent.click(screen.getByTestId('run-with-opensysml-menu'));
    expect(screen.getByText('Run with OpenSysML: Car')).toBeInTheDocument();
  });

  it('opens the publish dialog for the publish entry', () => {
    render(
      <MockedProvider>
        <RunWithOpenSysMLMenuContribution {...props} entry={{ id: 'publishWithOpenSysML' } as never} />
      </MockedProvider>
    );
    fireEvent.click(screen.getByTestId('publish-with-opensysml-menu'));
    expect(screen.getByText('Publish to SysML v2 API')).toBeInTheDocument();
  });
});
