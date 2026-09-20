import CheckIcon from '@mui/icons-material/Check';
import ClearIcon from '@mui/icons-material/Clear';
import ErrorIcon from '@mui/icons-material/Error';
import InfoIcon from '@mui/icons-material/Info';
import WarningIcon from '@mui/icons-material/Warning';
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
import { GQLRunOperation, GQLOpenSysMLRunResult } from '../graphql/runWithOpenSysML';

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

      {result.outputs.length > 0 && (
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

      {result.trace.length > 0 && (
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
            <div key={index}>
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
                    {verdict.subject} {verdict.kind}{' '}
                    {!verdict.decided ? (
                      <Chip label="undecided" size="small" />
                    ) : verdict.holds ? (
                      <CheckIcon color="success" />
                    ) : (
                      <ClearIcon color="error" />
                    )}{' '}
                    {verdict.detail ?? ''}
                  </>
                );
                return (
                  <TableRow
                    key={`${verdict.subject}-${index}`}
                    hover={Boolean(verdict.siriusId)}
                    onClick={() => verdict.siriusId && onSelectElement?.(verdict.siriusId)}>
                    <TableCell>{verdict.subject}</TableCell>
                    <TableCell>{verdict.kind}</TableCell>
                    <TableCell>
                      {!verdict.decided ? (
                        <Chip label="undecided" size="small" />
                      ) : verdict.holds ? (
                        <CheckIcon color="success" />
                      ) : (
                        <ClearIcon color="error" />
                      )}
                    </TableCell>
                    <TableCell>{content}</TableCell>
                  </TableRow>
                );
              })}
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
              const text = `${diagnostic.message}${
                diagnostic.documentName && diagnostic.line !== null
                  ? ` (${diagnostic.documentName}:${diagnostic.line})`
                  : ''
              }${diagnostic.qualifiedName ? ` [${diagnostic.qualifiedName}]` : ''}`;
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
