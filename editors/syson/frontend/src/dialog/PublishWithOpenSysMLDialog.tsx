import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Checkbox from '@mui/material/Checkbox';
import CircularProgress from '@mui/material/CircularProgress';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import FormControlLabel from '@mui/material/FormControlLabel';
import TextField from '@mui/material/TextField';
import { useEffect, useState } from 'react';
import { GQLPublishWithOpenSysMLInput, usePublishWithOpenSysML } from '../graphql/publishWithOpenSysML';
import { lastSeenHead, rememberedTarget, rememberPublication } from './publishMemory';

export interface PublishWithOpenSysMLDialogProps {
  editingContextId: string;
  onClose: () => void;
}

/** Publishes the whole model of an editing context to the SysML v2 API server the backend is configured with. */
export const PublishWithOpenSysMLDialog = ({ editingContextId, onClose }: PublishWithOpenSysMLDialogProps) => {
  const [remembered] = useState(() => rememberedTarget(editingContextId));
  const [project, setProject] = useState(remembered.project);
  const [projectName, setProjectName] = useState('');
  const [branch, setBranch] = useState(remembered.branch);
  const [message, setMessage] = useState('');
  const [token, setToken] = useState('');
  const [deleteMissing, setDeleteMissing] = useState(false);
  const { publish, loading, input, result } = usePublishWithOpenSysML();

  useEffect(() => {
    if (result?.ok && input) {
      rememberPublication(editingContextId, result.server, result.project, input.branch, result.branch, result.head);
      // A created project is published to by its id from now on.
      setProject(result.project);
      setProjectName('');
    }
  }, [editingContextId, input, result]);

  const namesProject = project.trim() !== '' || projectName.trim() !== '';
  const submit = (overwriteMoved: boolean): void => {
    const target = { project: project.trim(), branch: branch.trim() };
    const next: GQLPublishWithOpenSysMLInput = {
      id: crypto.randomUUID(),
      editingContextId,
      project: target.project,
      projectName: target.project === '' ? projectName.trim() : '',
      branch: target.branch,
      message,
      deleteMissing,
      lastSeenCommit: overwriteMoved ? null : lastSeenHead(editingContextId, target),
      token,
    };
    publish(next);
  };
  // The offer to publish over a moved branch holds only for the publication that was refused.
  const moved =
    result?.branchMoved === true &&
    input !== null &&
    input.project === project.trim() &&
    input.branch === branch.trim() &&
    input.deleteMissing === deleteMissing;
  const errors = result?.diagnostics.filter((diagnostic) => diagnostic.severity === 'error') ?? [];

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Publish to SysML v2 API</DialogTitle>
      <DialogContent>
        <TextField
          fullWidth
          margin="normal"
          label="Project ID"
          helperText="Leave empty to create a project"
          value={project}
          onChange={(event) => setProject(event.target.value)}
        />
        {project.trim() === '' && (
          <TextField
            fullWidth
            margin="normal"
            label="New project name"
            value={projectName}
            onChange={(event) => setProjectName(event.target.value)}
          />
        )}
        <TextField
          fullWidth
          margin="normal"
          label="Branch ID"
          helperText="Leave empty for the project's default branch"
          value={branch}
          onChange={(event) => setBranch(event.target.value)}
        />
        <TextField
          fullWidth
          margin="normal"
          label="Commit message"
          value={message}
          onChange={(event) => setMessage(event.target.value)}
        />
        <TextField
          fullWidth
          margin="normal"
          type="password"
          autoComplete="off"
          label="Bearer token"
          helperText="Leave empty to use the server's configured token; it is not kept"
          value={token}
          onChange={(event) => setToken(event.target.value)}
        />
        <FormControlLabel
          control={<Checkbox checked={deleteMissing} onChange={(event) => setDeleteMissing(event.target.checked)} />}
          label="Delete elements the branch holds beyond the model"
        />

        {result && (
          <Alert
            data-testid="publish-result"
            severity={result.ok ? 'success' : result.branchMoved ? 'warning' : 'error'}
            sx={{ whiteSpace: 'pre-line' }}>
            {result.summary}
            {'\n'}
            Server: {result.server}
          </Alert>
        )}
        {errors.length > 0 && (
          <ul data-testid="publish-errors">
            {errors.map((diagnostic, index) => (
              <li key={`${index}-${diagnostic.message}`}>
                {diagnostic.qualifiedName ? `${diagnostic.qualifiedName}: ` : ''}
                {diagnostic.message}
              </li>
            ))}
          </ul>
        )}
      </DialogContent>
      <DialogActions>
        {moved && (
          <Button onClick={() => submit(true)} disabled={loading} color="warning">
            Publish over the moved branch
          </Button>
        )}
        <Button onClick={() => submit(false)} disabled={loading || !namesProject} variant="contained">
          {loading && <CircularProgress size={20} />}
          Publish
        </Button>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};
