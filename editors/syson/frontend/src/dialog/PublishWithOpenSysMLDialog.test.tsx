import { MockedProvider } from '@apollo/client/testing';
import { ToastContext } from '@eclipse-sirius/sirius-components-core';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { GQLPublishWithOpenSysMLInput, publishWithOpenSysMLMutation } from '../graphql/publishWithOpenSysML';
import { PublishWithOpenSysMLDialog } from './PublishWithOpenSysMLDialog';
import { lastSeenHead, rememberPublication } from './publishMemory';

const SERVER = 'http://localhost:8083';

const published = {
  ok: true,
  branchMoved: false,
  summary: 'Published to branch main of project p1 as commit c2: 2 created, 0 updated, 0 deleted.',
  server: SERVER,
  project: 'p1',
  projectName: 'Mission',
  createdProject: false,
  branch: 'main',
  commit: 'c2',
  head: 'c2',
  created: 2,
  updated: 0,
  deleted: 0,
  leftInPlace: 0,
  diagnostics: [],
};

const answer = (matches: (input: GQLPublishWithOpenSysMLInput) => boolean, result: object) => ({
  request: { query: publishWithOpenSysMLMutation },
  variableMatcher: (variables: { input: GQLPublishWithOpenSysMLInput }) => matches(variables.input),
  result: {
    data: {
      publishWithOpenSysML: { __typename: 'PublishWithOpenSysMLSuccessPayload', id: 'publish', messages: [], result },
    },
  },
});

const renderDialog = (mocks: object[], enqueueSnackbar = vi.fn()) =>
  render(
    <ToastContext.Provider value={{ enqueueSnackbar }}>
      <MockedProvider mocks={mocks as never}>
        <PublishWithOpenSysMLDialog editingContextId="ctx" onClose={vi.fn()} />
      </MockedProvider>
    </ToastContext.Provider>
  );

const type = (label: string, value: string) => fireEvent.change(screen.getByLabelText(label), { target: { value } });
const publish = () => fireEvent.click(screen.getByRole('button', { name: 'Publish' }));

describe('PublishWithOpenSysMLDialog', () => {
  beforeEach(() => window.localStorage.clear());

  it('needs a project id or the name of a project to create', () => {
    renderDialog([]);
    expect(screen.getByRole('button', { name: 'Publish' })).toBeDisabled();
    type('New project name', 'Rover');
    expect(screen.getByRole('button', { name: 'Publish' })).toBeEnabled();
    type('Project ID', 'p1');
    expect(screen.queryByLabelText('New project name')).not.toBeInTheDocument();
  });

  it('publishes, shows what was written and remembers the head but never the token', async () => {
    renderDialog([
      answer(
        (input) =>
          input.editingContextId === 'ctx' &&
          input.project === 'p1' &&
          input.projectName === '' &&
          input.branch === '' &&
          input.message === 'first' &&
          input.token === 'secret' &&
          input.deleteMissing &&
          input.lastSeenCommit === null,
        published
      ),
    ]);
    type('Project ID', ' p1 ');
    type('Commit message', 'first');
    type('Bearer token', 'secret');
    fireEvent.click(screen.getByRole('checkbox'));
    publish();

    await waitFor(() => expect(screen.getByTestId('publish-result')).toHaveTextContent('2 created'));
    expect(screen.getByTestId('publish-result')).toHaveTextContent(`Server: ${SERVER}`);
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBe('c2');
    expect(JSON.stringify({ ...window.localStorage })).not.toContain('secret');
  });

  it('offers the last target again and sends the head it last saw', async () => {
    rememberPublication('ctx', SERVER, 'p1', '', 'main', 'c1');
    renderDialog([answer((input) => input.project === 'p1' && input.lastSeenCommit === 'c1', published)]);
    expect(screen.getByLabelText('Project ID')).toHaveValue('p1');
    publish();

    await waitFor(() => expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBe('c2'));
  });

  it('publishes over a moved branch only when asked to', async () => {
    rememberPublication('ctx', SERVER, 'p1', '', 'main', 'c1');
    const moved = { ...published, ok: false, branchMoved: true, summary: 'branch main moved', commit: null, head: '' };
    renderDialog([
      answer((input) => input.lastSeenCommit === 'c1', moved),
      answer((input) => input.lastSeenCommit === null, published),
    ]);
    publish();

    await waitFor(() => expect(screen.getByTestId('publish-result')).toHaveTextContent('branch main moved'));
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBe('c1');
    fireEvent.click(screen.getByRole('button', { name: 'Publish over the moved branch' }));

    await waitFor(() => expect(screen.getByTestId('publish-result')).toHaveTextContent('2 created'));
    expect(screen.queryByRole('button', { name: 'Publish over the moved branch' })).not.toBeInTheDocument();
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBe('c2');
  });

  it('withdraws the offer to publish over a moved branch once the target changes', async () => {
    rememberPublication('ctx', SERVER, 'p1', '', 'main', 'c1');
    const moved = { ...published, ok: false, branchMoved: true, summary: 'branch main moved', commit: null, head: '' };
    renderDialog([answer(() => true, moved)]);
    publish();

    await waitFor(() => expect(screen.getByRole('button', { name: 'Publish over the moved branch' })).toBeEnabled());
    type('Branch ID', 'dev');
    expect(screen.queryByRole('button', { name: 'Publish over the moved branch' })).not.toBeInTheDocument();
  });

  it('lists the errors of a model that was refused', async () => {
    const refused = {
      ...published,
      ok: false,
      summary: 'The model has 1 error(s) and was not published; the first: cannot resolve B',
      commit: null,
      head: '',
      diagnostics: [
        {
          severity: 'error',
          message: 'cannot resolve B',
          code: 'E1',
          documentName: null,
          line: null,
          qualifiedName: 'P::A',
          elementId: null,
          siriusId: null,
        },
        {
          severity: 'warning',
          message: 'unused',
          code: 'W1',
          documentName: null,
          line: null,
          qualifiedName: null,
          elementId: null,
          siriusId: null,
        },
      ],
    };
    renderDialog([answer(() => true, refused)]);
    type('Project ID', 'p1');
    publish();

    await waitFor(() => expect(screen.getByTestId('publish-errors')).toHaveTextContent('P::A: cannot resolve B'));
    expect(screen.getByTestId('publish-errors')).not.toHaveTextContent('unused');
    expect(lastSeenHead('ctx', { project: 'p1', branch: '' })).toBeNull();
  });

  it('reports a server failure as a toast', async () => {
    const enqueueSnackbar = vi.fn();
    renderDialog(
      [
        {
          request: { query: publishWithOpenSysMLMutation },
          variableMatcher: () => true,
          result: {
            data: {
              publishWithOpenSysML: {
                __typename: 'ErrorPayload',
                messages: [{ body: 'the server answered 401', level: 'ERROR' }],
              },
            },
          },
        },
      ],
      enqueueSnackbar
    );
    type('Project ID', 'p1');
    publish();

    await waitFor(() => expect(enqueueSnackbar).toHaveBeenCalledWith('the server answered 401', { variant: 'error' }));
    expect(screen.queryByTestId('publish-result')).not.toBeInTheDocument();
  });
});
