import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { RunResultsPanel } from './RunResultsPanel';
import type { GQLOpenSysMLRunResult } from '../graphql/runWithOpenSysML';

const baseResult = (overrides: Partial<GQLOpenSysMLRunResult> = {}): GQLOpenSysMLRunResult => ({
  modelHash: '12345678901234567890',
  operation: 'INSTANTIATE',
  target: 'Vehicle::Car',
  ok: true,
  verdict: null,
  schedule: null,
  finalTime: null,
  outputs: [],
  trace: [],
  resultText: null,
  diagnostics: [],
  verdicts: [],
  instances: [],
  ...overrides,
});

describe('RunResultsPanel', () => {
  it('renders verification verdicts', () => {
    render(
      <RunResultsPanel
        result={baseResult({
          operation: 'VERIFY_REQUIREMENT',
          ok: false,
          verdict: 'violated',
          verdicts: [{ subject: 'Req', kind: 'requirement', holds: false, detail: 'broken', siriusId: null }],
        })}
      />
    );
    expect(screen.getByText('violated')).toBeInTheDocument();
    expect(screen.getByText('Req')).toBeInTheDocument();
    expect(screen.getByRole('cell', { name: /broken/ })).toBeInTheDocument();
  });

  it('renders outputs, schedule, and final time', () => {
    render(
      <RunResultsPanel
        result={baseResult({
          operation: 'EXECUTE_ACTION',
          schedule: 'declared',
          finalTime: 2,
          outputs: [{ name: 'y', value: '42' }],
        })}
      />
    );
    expect(screen.getByText('y')).toBeInTheDocument();
    expect(screen.getByText('42')).toBeInTheDocument();
    expect(screen.getByText('schedule: declared')).toBeInTheDocument();
    expect(screen.getByText('final time: 2')).toBeInTheDocument();
  });

  it('selects mapped diagnostics but leaves unmapped diagnostics as plain list items', () => {
    const onSelectElement = vi.fn();
    render(
      <RunResultsPanel
        result={baseResult({
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
            {
              severity: 'warning',
              message: 'unmapped',
              code: '',
              documentName: null,
              line: null,
              qualifiedName: null,
              elementId: null,
              siriusId: null,
            },
          ],
        })}
        onSelectElement={onSelectElement}
      />
    );
    const diagnostics = screen.getAllByTestId('opensysml-diagnostic');
    expect(diagnostics).toHaveLength(2);
    fireEvent.click(diagnostics[0]);
    expect(onSelectElement).toHaveBeenCalledWith('sid-1');
    expect(screen.getByRole('button', { name: /mapped/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /unmapped/i })).not.toBeInTheDocument();
  });

  it('renders the empty diagnostic state', () => {
    render(<RunResultsPanel result={baseResult()} />);
    expect(screen.getByText('completed')).toBeInTheDocument();
    expect(screen.getByText('No diagnostics.')).toBeInTheDocument();
  });

  it('renders instances and feature values', () => {
    render(
      <RunResultsPanel
        result={baseResult({
          instances: [
            { id: 1, type: 'Vehicle::Car', typeSiriusId: 'car-id', featureValues: [{ name: 'speed', value: '42' }] },
          ],
        })}
      />
    );
    expect(screen.getByText('Vehicle::Car')).toBeInTheDocument();
    expect(screen.getByText('speed = 42')).toBeInTheDocument();
  });
});
