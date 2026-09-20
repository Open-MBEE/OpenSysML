import { MockedProvider } from '@apollo/client/testing';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { ToastContext, SelectionContext } from '@eclipse-sirius/sirius-components-core';
import { describe, expect, it, vi } from 'vitest';
import { runWithOpenSysMLMutation } from '../graphql/runWithOpenSysML';
import { RunWithOpenSysMLDialog } from './RunWithOpenSysMLDialog';

const successResult = {
  modelHash: 'hash',
  operation: 'EXECUTE_ACTION',
  target: 'Counter',
  ok: true,
  verdict: null,
  schedule: 'declared',
  finalTime: 1,
  outputs: [{ name: 'y', value: '42' }],
  trace: [],
  outcomes: [],
  resultText: null,
  diagnostics: [],
  verdicts: [],
  instances: [],
};

type RunVariables = {
  input: {
    editingContextId: string;
    objectId: string;
    operation: string;
    inputs: Array<{ name: string; expression: string }>;
  };
};

const mocks = [
  {
    request: {
      query: runWithOpenSysMLMutation,
    },
    variableMatcher: (variables: RunVariables) =>
      variables.input.editingContextId === 'ctx' &&
      variables.input.objectId === 'obj' &&
      variables.input.operation === 'EXECUTE_ACTION' &&
      variables.input.inputs[0]?.name === 'x' &&
      variables.input.inputs[0]?.expression === '21',
    result: {
      data: {
        runWithOpenSysML: {
          __typename: 'RunWithOpenSysMLSuccessPayload',
          id: 'run',
          messages: [],
          result: successResult,
        },
      },
    },
  },
];

const props = { editingContextId: 'ctx', objectId: 'obj', elementLabel: 'Counter', onClose: vi.fn() };

describe('RunWithOpenSysMLDialog', () => {
  it('runs an action with inputs and renders its result', async () => {
    render(
      <MockedProvider mocks={mocks}>
        <RunWithOpenSysMLDialog {...props} />
      </MockedProvider>
    );
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Operation' }));
    fireEvent.click(screen.getByRole('option', { name: 'Execute action' }));
    fireEvent.click(screen.getByText('Add input'));
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'x' } });
    fireEvent.change(screen.getByLabelText('Expression'), { target: { value: '21' } });
    fireEvent.click(screen.getByRole('button', { name: 'Run' }));
    await waitFor(() => expect(screen.getByText('42')).toBeInTheDocument());
  });

  it('reports an ErrorPayload through the toast context', async () => {
    const enqueueSnackbar = vi.fn();
    const errorMock = {
      request: { query: runWithOpenSysMLMutation },
      variableMatcher: () => true,
      result: {
        data: {
          runWithOpenSysML: {
            __typename: 'ErrorPayload',
            id: null,
            messages: [{ body: 'bad model', level: 'ERROR' }],
          },
        },
      },
    };
    render(
      <ToastContext.Provider value={{ enqueueSnackbar }}>
        <MockedProvider mocks={[errorMock]}>
          <RunWithOpenSysMLDialog {...props} />
        </MockedProvider>
      </ToastContext.Provider>
    );
    fireEvent.click(screen.getByRole('button', { name: 'Run' }));
    await waitFor(() => expect(enqueueSnackbar).toHaveBeenCalledWith('bad model', { variant: 'error' }));
  });

  it('selects a mapped diagnostic without closing the dialog', async () => {
    const setSelection = vi.fn();
    const diagnosticMock = {
      request: { query: runWithOpenSysMLMutation },
      variableMatcher: () => true,
      result: {
        data: {
          runWithOpenSysML: {
            __typename: 'RunWithOpenSysMLSuccessPayload',
            id: 'run',
            messages: [],
            result: {
              ...successResult,
              diagnostics: [
                {
                  severity: 'error',
                  message: 'mapped',
                  code: '',
                  documentName: null,
                  line: null,
                  qualifiedName: null,
                  elementId: null,
                  siriusId: 'sid-1',
                },
              ],
            },
          },
        },
      },
    };
    render(
      <SelectionContext.Provider value={{ selection: { entries: [] }, setSelection }}>
        <MockedProvider mocks={[diagnosticMock]}>
          <RunWithOpenSysMLDialog {...props} />
        </MockedProvider>
      </SelectionContext.Provider>
    );
    fireEvent.click(screen.getByRole('button', { name: 'Run' }));
    await waitFor(() => expect(screen.getByText(/mapped/)).toBeInTheDocument());
    fireEvent.click(screen.getByTestId('opensysml-diagnostic'));
    expect(setSelection).toHaveBeenCalledWith({ entries: [{ id: 'sid-1' }] });
    expect(screen.getByText('Run with OpenSysML: Counter')).toBeInTheDocument();
  });

  it('submits ordered event and argument entries', async () => {
    type OrderedVariables = { input: { events: string[]; arguments: string[] } };
    const variableMatcher = vi.fn((_variables: OrderedVariables) => true);
    const entryMock = {
      request: { query: runWithOpenSysMLMutation },
      variableMatcher,
      result: {
        data: {
          runWithOpenSysML: {
            __typename: 'RunWithOpenSysMLSuccessPayload',
            id: 'run',
            messages: [],
            result: successResult,
          },
        },
      },
    };
    render(
      <MockedProvider mocks={[entryMock]}>
        <RunWithOpenSysMLDialog {...props} />
      </MockedProvider>
    );
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Operation' }));
    fireEvent.click(screen.getByRole('option', { name: 'Execute state machine' }));
    fireEvent.click(screen.getByText('Add event'));
    fireEvent.change(screen.getByLabelText('Event 1'), { target: { value: 'start' } });
    fireEvent.click(screen.getByText('Add event'));
    fireEvent.change(screen.getByLabelText('Event 2'), { target: { value: 'stop' } });
    fireEvent.click(screen.getByRole('combobox', { name: 'Operation' }));
    fireEvent.click(screen.getByRole('option', { name: 'Evaluate calculation' }));
    fireEvent.click(screen.getByText('Add argument'));
    fireEvent.change(screen.getByLabelText('Argument 1'), { target: { value: '1' } });
    fireEvent.click(screen.getByText('Add argument'));
    fireEvent.change(screen.getByLabelText('Argument 2'), { target: { value: '2' } });
    fireEvent.click(screen.getByRole('button', { name: 'Run' }));
    await waitFor(() => expect(variableMatcher).toHaveBeenCalled());
    const variables = variableMatcher.mock.calls[0][0];
    expect(variables.input.events).toEqual(['start', 'stop']);
    expect(variables.input.arguments).toEqual(['1', '2']);
  });

  it('disables Run when input names are duplicated', () => {
    render(
      <MockedProvider mocks={[]}>
        <RunWithOpenSysMLDialog {...props} />
      </MockedProvider>
    );
    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Operation' }));
    fireEvent.click(screen.getByRole('option', { name: 'Execute action' }));
    fireEvent.click(screen.getByText('Add input'));
    fireEvent.click(screen.getByText('Add input'));
    fireEvent.change(screen.getAllByLabelText('Name')[0], { target: { value: 'x' } });
    fireEvent.change(screen.getAllByLabelText('Name')[1], { target: { value: 'x' } });

    expect(screen.getAllByText('Duplicate input name')).not.toHaveLength(0);
    expect(screen.getByRole('button', { name: 'Run' })).toBeDisabled();
  });
});
