import AddIcon from '@mui/icons-material/Add';
import DeleteIcon from '@mui/icons-material/Delete';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import FormControl from '@mui/material/FormControl';
import InputLabel from '@mui/material/InputLabel';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import TextField from '@mui/material/TextField';
import { useState } from 'react';
import { useSelection } from '@eclipse-sirius/sirius-components-core';
import { GQLRunInputValue, GQLRunOperation, useRunWithOpenSysML } from '../graphql/runWithOpenSysML';
import { RunResultsPanel } from '../results/RunResultsPanel';

export interface RunWithOpenSysMLDialogProps {
  editingContextId: string;
  objectId: string;
  elementLabel: string;
  onClose: () => void;
}

const operationOptions: Array<[GQLRunOperation, string]> = [
  ['INSTANTIATE', 'Instantiate'],
  ['EXECUTE_ACTION', 'Execute action'],
  ['EXPLORE_ACTION', 'Explore action'],
  ['EXECUTE_STATE', 'Execute state machine'],
  ['EXPLORE_STATE', 'Explore state machine'],
  ['VERIFY_CONSTRAINT', 'Verify constraint'],
  ['VERIFY_REQUIREMENT', 'Verify requirement'],
  ['VERIFY_SATISFACTION', 'Verify satisfaction'],
  ['EVALUATE_CALC', 'Evaluate calculation'],
  ['RUN_ANALYSIS', 'Run analysis'],
  ['VALIDATE_INSTANCE', 'Validate instance'],
];

const usesInputs = (operation: GQLRunOperation) =>
  operation === 'EXECUTE_ACTION' || operation === 'EXPLORE_ACTION' || operation === 'RUN_ANALYSIS';
const usesEvents = (operation: GQLRunOperation) => operation === 'EXECUTE_STATE' || operation === 'EXPLORE_STATE';
const usesArguments = (operation: GQLRunOperation) => operation === 'EVALUATE_CALC' || operation === 'RUN_ANALYSIS';
const usesSchedule = (operation: GQLRunOperation) =>
  operation === 'EXECUTE_ACTION' ||
  operation === 'EXPLORE_ACTION' ||
  operation === 'EXECUTE_STATE' ||
  operation === 'EXPLORE_STATE' ||
  operation === 'RUN_ANALYSIS';
const usesSubject = (operation: GQLRunOperation) =>
  operation === 'VERIFY_CONSTRAINT' ||
  operation === 'VERIFY_REQUIREMENT' ||
  operation === 'VERIFY_SATISFACTION' ||
  operation === 'RUN_ANALYSIS';

export const RunWithOpenSysMLDialog = ({
  editingContextId,
  objectId,
  elementLabel,
  onClose,
}: RunWithOpenSysMLDialogProps) => {
  const [operation, setOperation] = useState<GQLRunOperation>('INSTANTIATE');
  const [inputs, setInputs] = useState<GQLRunInputValue[]>([]);
  const [events, setEvents] = useState<string[]>([]);
  const [argumentsText, setArgumentsText] = useState<string[]>([]);
  const [schedule, setSchedule] = useState('');
  const [subject, setSubject] = useState('');
  const { run, loading, result } = useRunWithOpenSysML();
  const { setSelection } = useSelection();
  const hasInvalidInputNames = inputs.some((input) => input.name.trim() === '');
  const hasDuplicateInputNames = inputs.some(
    (input, index) => input.name !== '' && inputs.findIndex((entry) => entry.name === input.name) !== index
  );

  const submit = (): void => {
    run({
      id: crypto.randomUUID(),
      editingContextId,
      objectId,
      operation,
      inputs,
      events: events.filter(Boolean),
      arguments: argumentsText.filter(Boolean),
      schedule: schedule || null,
      subject: subject || null,
    });
  };

  const selectElement = (siriusId: string): void => {
    setSelection({ entries: [{ id: siriusId }] });
  };

  return (
    <Dialog open onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>Run with OpenSysML: {elementLabel}</DialogTitle>
      <DialogContent>
        <FormControl fullWidth margin="normal">
          <InputLabel id="opensysml-operation-label">Operation</InputLabel>
          <Select
            labelId="opensysml-operation-label"
            aria-label="Operation"
            label="Operation"
            value={operation}
            onChange={(event) => setOperation(event.target.value as GQLRunOperation)}>
            {operationOptions.map(([value, label]) => (
              <MenuItem key={value} value={value}>
                {label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        {usesInputs(operation) && (
          <div>
            {inputs.map((input, index) => (
              <div key={`${index}-${input.name}`}>
                <TextField
                  label="Name"
                  value={input.name}
                  error={
                    input.name.trim() === '' ||
                    (input.name !== '' && inputs.findIndex((entry) => entry.name === input.name) !== index)
                  }
                  helperText={
                    input.name.trim() === ''
                      ? 'Input name is required'
                      : input.name !== '' && inputs.findIndex((entry) => entry.name === input.name) !== index
                      ? 'Duplicate input name'
                      : undefined
                  }
                  onChange={(event) =>
                    setInputs((previous) =>
                      previous.map((entry, entryIndex) =>
                        entryIndex === index ? { ...entry, name: event.target.value } : entry
                      )
                    )
                  }
                />
                <TextField
                  label="Expression"
                  value={input.expression}
                  onChange={(event) =>
                    setInputs((previous) =>
                      previous.map((entry, entryIndex) =>
                        entryIndex === index ? { ...entry, expression: event.target.value } : entry
                      )
                    )
                  }
                />
                <Button
                  aria-label={`Remove input ${index + 1}`}
                  onClick={() => setInputs((previous) => previous.filter((_, entryIndex) => entryIndex !== index))}>
                  <DeleteIcon />
                </Button>
              </div>
            ))}
            <Button
              startIcon={<AddIcon />}
              onClick={() => setInputs((previous) => [...previous, { name: '', expression: '' }])}>
              Add input
            </Button>
          </div>
        )}

        {usesEvents(operation) && (
          <div>
            {events.map((event, index) => (
              <div key={`${index}-${event}`}>
                <TextField
                  fullWidth
                  margin="normal"
                  label={`Event ${index + 1}`}
                  value={event}
                  onChange={(change) =>
                    setEvents((previous) =>
                      previous.map((entry, entryIndex) => (entryIndex === index ? change.target.value : entry))
                    )
                  }
                />
                <Button
                  aria-label={`Remove event ${index + 1}`}
                  onClick={() => setEvents((previous) => previous.filter((_, entryIndex) => entryIndex !== index))}>
                  <DeleteIcon />
                </Button>
              </div>
            ))}
            <Button startIcon={<AddIcon />} onClick={() => setEvents((previous) => [...previous, ''])}>
              Add event
            </Button>
          </div>
        )}

        {usesArguments(operation) && (
          <div>
            {argumentsText.map((argument, index) => (
              <div key={`${index}-${argument}`}>
                <TextField
                  fullWidth
                  margin="normal"
                  label={`Argument ${index + 1}`}
                  value={argument}
                  onChange={(change) =>
                    setArgumentsText((previous) =>
                      previous.map((entry, entryIndex) => (entryIndex === index ? change.target.value : entry))
                    )
                  }
                />
                <Button
                  aria-label={`Remove argument ${index + 1}`}
                  onClick={() =>
                    setArgumentsText((previous) => previous.filter((_, entryIndex) => entryIndex !== index))
                  }>
                  <DeleteIcon />
                </Button>
              </div>
            ))}
            <Button startIcon={<AddIcon />} onClick={() => setArgumentsText((previous) => [...previous, ''])}>
              Add argument
            </Button>
          </div>
        )}

        {usesSchedule(operation) && (
          <TextField
            fullWidth
            margin="normal"
            label="Schedule"
            placeholder="declared, seed:7, random, explore:runs=10,depth=100"
            value={schedule}
            onChange={(event) => setSchedule(event.target.value)}
          />
        )}

        {usesSubject(operation) && (
          <TextField
            fullWidth
            margin="normal"
            label="Subject"
            value={subject}
            onChange={(event) => setSubject(event.target.value)}
          />
        )}

        {result && <RunResultsPanel result={result} onSelectElement={selectElement} />}
      </DialogContent>
      <DialogActions>
        <Button
          onClick={submit}
          disabled={loading || hasInvalidInputNames || hasDuplicateInputNames}
          variant="contained">
          {loading && <CircularProgress size={20} />}
          Run
        </Button>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};
