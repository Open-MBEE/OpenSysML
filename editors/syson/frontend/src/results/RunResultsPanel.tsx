import CheckIcon from '@mui/icons-material/Check';
import DownloadIcon from '@mui/icons-material/Download';
import ClearIcon from '@mui/icons-material/Clear';
import ErrorIcon from '@mui/icons-material/Error';
import InfoIcon from '@mui/icons-material/Info';
import WarningIcon from '@mui/icons-material/Warning';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import List from '@mui/material/List';
import ListItem from '@mui/material/ListItem';
import ListItemButton from '@mui/material/ListItemButton';
import ListItemText from '@mui/material/ListItemText';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import {
  GQLOpenSysMLDocument,
  GQLOpenSysMLRunOutcome,
  GQLOpenSysMLRunResult,
  GQLRunOperation,
} from '../graphql/runWithOpenSysML';

export interface RunResultsPanelProps {
  result: GQLOpenSysMLRunResult;
  onSelectElement?: (siriusId: string) => void;
}

const operationLabels: Record<GQLRunOperation, string> = {
  INSTANTIATE: 'Instantiate',
  EXECUTE_ACTION: 'Execute action',
  EXPLORE_ACTION: 'Explore action',
  EXECUTE_STATE: 'Execute state machine',
  EXPLORE_STATE: 'Explore state machine',
  VERIFY_CONSTRAINT: 'Verify constraint',
  VERIFY_REQUIREMENT: 'Verify requirement',
  VERIFY_SATISFACTION: 'Verify satisfaction',
  EVALUATE_CALC: 'Evaluate calculation',
  RUN_ANALYSIS: 'Run analysis',
  VALIDATE_INSTANCE: 'Validate instance',
  RENDER_DOCUMENT: 'Render document',
  RUN_DOCUMENT_QUERY: 'Run document query',
};

const documentMediaType = (document: GQLOpenSysMLDocument): string =>
  document.form === 'HTML' ? 'text/html;charset=utf-8' : 'text/markdown;charset=utf-8';

// Offers the rendered document as a file download through a transient object URL.
const downloadDocument = (document: GQLOpenSysMLDocument): void => {
  const url = URL.createObjectURL(new Blob([document.content], { type: documentMediaType(document) }));
  const anchor = window.document.createElement('a');
  anchor.href = url;
  anchor.download = document.fileName;
  anchor.click();
  URL.revokeObjectURL(url);
};

const verdictColor = (result: GQLOpenSysMLRunResult): 'success' | 'error' | 'warning' => {
  const status = result.verdict ?? (result.ok ? 'completed' : 'failed');
  if (status === 'holds' || status === 'pass' || status === 'completed') {
    return 'success';
  }
  if (status === 'violated' || status === 'fail' || status === 'failed') {
    return 'error';
  }
  return 'warning';
};

const verdictLabel = (result: GQLOpenSysMLRunResult): string => result.verdict ?? (result.ok ? 'completed' : 'failed');

const outcomeKeys = (outcomes: GQLOpenSysMLRunOutcome[]): string[] => {
  const seen = new Map<string, number>();
  return outcomes.map((outcome) => {
    const base = JSON.stringify(outcome);
    const count = seen.get(base) ?? 0;
    seen.set(base, count + 1);
    return count === 0 ? base : `${base}#${count}`;
  });
};

const verdictIcon = (verdict: GQLOpenSysMLRunResult['verdicts'][number]) => {
  if (!verdict.decided) {
    return <Chip label="undecided" size="small" />;
  }
  return verdict.holds ? <CheckIcon color="success" /> : <ClearIcon color="error" />;
};

const diagnosticIcon = (severity: string) => {
  switch (severity.toLowerCase()) {
    case 'error':
      return <ErrorIcon color="error" fontSize="small" />;
    case 'warning':
      return <WarningIcon color="warning" fontSize="small" />;
    default:
      return <InfoIcon color="info" fontSize="small" />;
  }
};

export const RunResultsPanel = ({ result, onSelectElement }: RunResultsPanelProps) => {
  const status = verdictLabel(result);
  const keys = outcomeKeys(result.outcomes);
  return (
    <div>
      <Typography component="div" variant="subtitle1">
        <Chip label={status} color={verdictColor(result)} size="small" /> {operationLabels[result.operation]} on{' '}
        {result.target}
      </Typography>
      <Typography title={result.modelHash} variant="caption">
        {result.modelHash.slice(0, 12)}
      </Typography>
      {result.schedule && <Typography variant="body2">schedule: {result.schedule}</Typography>}
      {result.finalTime !== null && <Typography variant="body2">final time: {result.finalTime}</Typography>}
      {result.resultText && <Typography variant="body2">{result.resultText}</Typography>}

      {result.outcomes.length === 0 && result.outputs.length > 0 && (
        <section>
          <Typography variant="subtitle2">Outputs</Typography>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Value</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {result.outputs.map((output) => (
                <TableRow key={output.name}>
                  <TableCell>{output.name}</TableCell>
                  <TableCell>{output.value}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </section>
      )}

      {result.outcomes.length === 0 && result.trace.length > 0 && (
        <section>
          <Typography variant="subtitle2">Trace</Typography>
          <List dense>
            {result.trace.map((state, index) => (
              <ListItem key={`${state}-${index}`}>
                <ListItemText primary={state} />
              </ListItem>
            ))}
          </List>
        </section>
      )}

      {result.outcomes.length >= 1 && (
        <section>
          <Typography variant="subtitle2">Outcomes ({result.outcomes.length})</Typography>
          {result.outcomes.map((outcome, index) => (
            <div key={keys[index]}>
              {outcome.outputs.length > 0 && (
                <Typography variant="body2">
                  outputs: {outcome.outputs.map((output) => `${output.name} = ${output.value}`).join(', ')}
                </Typography>
              )}
              {outcome.trace.length > 0 && <Typography variant="body2">trace: {outcome.trace.join(', ')}</Typography>}
              {outcome.error && <Typography variant="body2">error: {outcome.error}</Typography>}
              {outcome.witness.length > 0 && (
                <Typography variant="body2">witness: {outcome.witness.join(', ')}</Typography>
              )}
              <Typography variant="body2">linearizations: {outcome.linearizations}</Typography>
            </div>
          ))}
        </section>
      )}

      {result.verdicts.length > 0 && (
        <section>
          <Typography variant="subtitle2">Verdicts</Typography>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Subject</TableCell>
                <TableCell>Kind</TableCell>
                <TableCell>Holds</TableCell>
                <TableCell>Detail</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {result.verdicts.map((verdict, index) => {
                const content = (
                  <>
                    {verdict.subject} {verdict.kind} {verdictIcon(verdict)} {verdict.detail ?? ''}
                  </>
                );
                return (
                  <TableRow
                    key={`${verdict.subject}-${index}`}
                    hover={Boolean(verdict.siriusId)}
                    onClick={() => verdict.siriusId && onSelectElement?.(verdict.siriusId)}>
                    <TableCell>{verdict.subject}</TableCell>
                    <TableCell>{verdict.kind}</TableCell>
                    <TableCell>{verdictIcon(verdict)}</TableCell>
                    <TableCell>{content}</TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </section>
      )}

      {result.document && (
        <section data-testid="opensysml-document">
          <Typography variant="subtitle2">
            Document ({result.document.form === 'HTML' ? 'HTML' : 'Markdown'})
            <Button
              size="small"
              startIcon={<DownloadIcon />}
              onClick={() => result.document && downloadDocument(result.document)}>
              Download {result.document.fileName}
            </Button>
          </Typography>
          {result.document.form === 'HTML' ? (
            <iframe
              title={result.document.fileName}
              sandbox=""
              srcDoc={result.document.content}
              style={{ width: '100%', height: '32rem', border: '1px solid rgba(0, 0, 0, 0.12)' }}
            />
          ) : (
            <Typography
              component="pre"
              variant="body2"
              style={{ whiteSpace: 'pre-wrap', maxHeight: '32rem', overflow: 'auto' }}>
              {result.document.content}
            </Typography>
          )}
        </section>
      )}

      {result.table && (
        <section data-testid="opensysml-table">
          <Typography variant="subtitle2">Rows</Typography>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Element</TableCell>
                {result.table.columns.map((column) => (
                  <TableCell key={column}>{column}</TableCell>
                ))}
              </TableRow>
            </TableHead>
            <TableBody>
              {result.table.rows.map((row, index) => (
                <TableRow
                  key={`${row.element}-${index}`}
                  hover={Boolean(row.siriusId)}
                  onClick={() => row.siriusId && onSelectElement?.(row.siriusId)}>
                  <TableCell>{row.element}</TableCell>
                  {row.cells.map((cell, cellIndex) => (
                    <TableCell key={`${result.table?.columns[cellIndex] ?? cellIndex}`}>{cell}</TableCell>
                  ))}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </section>
      )}

      {result.instances.length > 0 && (
        <section>
          <Typography variant="subtitle2">Instances</Typography>
          <List dense>
            {result.instances.map((instance) => (
              <ListItem key={instance.id}>
                <ListItemText
                  primary={
                    instance.typeSiriusId ? (
                      <button type="button" onClick={() => onSelectElement?.(instance.typeSiriusId as string)}>
                        {instance.type}
                      </button>
                    ) : (
                      instance.type
                    )
                  }
                  secondary={instance.featureValues.map((value) => `${value.name} = ${value.value}`).join(', ')}
                />
              </ListItem>
            ))}
          </List>
        </section>
      )}

      <section>
        <Typography variant="subtitle2">Diagnostics</Typography>
        {result.diagnostics.length === 0 ? (
          <Typography>No diagnostics.</Typography>
        ) : (
          <List dense>
            {result.diagnostics.map((diagnostic, index) => {
              const location =
                diagnostic.documentName && diagnostic.line !== null
                  ? ` (${diagnostic.documentName}:${diagnostic.line})`
                  : '';
              const qualified = diagnostic.qualifiedName ? ` [${diagnostic.qualifiedName}]` : '';
              const text = `${diagnostic.message}${location}${qualified}`;
              const primary = (
                <>
                  {diagnosticIcon(diagnostic.severity)} {text}
                </>
              );
              return diagnostic.siriusId ? (
                <ListItemButton
                  key={`${diagnostic.message}-${index}`}
                  data-testid="opensysml-diagnostic"
                  onClick={() => onSelectElement?.(diagnostic.siriusId as string)}>
                  <ListItemText primary={primary} />
                </ListItemButton>
              ) : (
                <ListItem key={`${diagnostic.message}-${index}`} data-testid="opensysml-diagnostic">
                  <ListItemText primary={primary} />
                </ListItem>
              );
            })}
          </List>
        )}
      </section>
    </div>
  );
};
