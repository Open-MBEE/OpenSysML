import { gql, useMutation } from '@apollo/client';
import { useMultiToast } from '@eclipse-sirius/sirius-components-core';
import { useState } from 'react';
import type { GQLOpenSysMLDiagnostic } from './runWithOpenSysML';

interface GQLMessage {
  body: string;
  level: string;
}

interface GQLErrorPayload {
  __typename: 'ErrorPayload';
  messages: GQLMessage[] | null;
}

export interface GQLPublishWithOpenSysMLInput {
  id: string;
  editingContextId: string;
  project: string;
  projectName: string;
  branch: string;
  message: string;
  deleteMissing: boolean;
  /** The commit this browser last saw the branch at; null publishes wherever the branch is. */
  lastSeenCommit: string | null;
  token: string;
}

export interface GQLOpenSysMLPublishResult {
  ok: boolean;
  branchMoved: boolean;
  summary: string;
  server: string;
  project: string;
  projectName: string;
  createdProject: boolean;
  branch: string;
  commit: string | null;
  head: string;
  created: number;
  updated: number;
  deleted: number;
  leftInPlace: number;
  diagnostics: GQLOpenSysMLDiagnostic[];
}

export interface GQLPublishWithOpenSysMLSuccessPayload {
  __typename: 'PublishWithOpenSysMLSuccessPayload';
  id: string;
  messages: GQLMessage[];
  result: GQLOpenSysMLPublishResult;
}

export type GQLPublishWithOpenSysMLPayload = GQLErrorPayload | GQLPublishWithOpenSysMLSuccessPayload;

interface GQLPublishWithOpenSysMLMutationData {
  publishWithOpenSysML: GQLPublishWithOpenSysMLPayload;
}

export const publishWithOpenSysMLMutation = gql`
  mutation publishWithOpenSysML($input: PublishWithOpenSysMLInput!) {
    publishWithOpenSysML(input: $input) {
      __typename
      ... on ErrorPayload {
        messages {
          body
          level
        }
      }
      ... on PublishWithOpenSysMLSuccessPayload {
        id
        messages {
          body
          level
        }
        result {
          ok
          branchMoved
          summary
          server
          project
          projectName
          createdProject
          branch
          commit
          head
          created
          updated
          deleted
          leftInPlace
          diagnostics {
            severity
            message
            code
            documentName
            line
            qualifiedName
            elementId
            siriusId
          }
        }
      }
    }
  }
`;

export interface UsePublishWithOpenSysMLValue {
  publish: (input: GQLPublishWithOpenSysMLInput) => void;
  loading: boolean;
  /** The publication the result answers, so what it saw can be kept under what was asked. */
  input: GQLPublishWithOpenSysMLInput | null;
  result: GQLOpenSysMLPublishResult | null;
}

interface Answer {
  input: GQLPublishWithOpenSysMLInput;
  result: GQLOpenSysMLPublishResult;
}

export const usePublishWithOpenSysML = (): UsePublishWithOpenSysMLValue => {
  const { addErrorMessage } = useMultiToast();
  const [performPublish, { loading }] = useMutation<
    GQLPublishWithOpenSysMLMutationData,
    { input: GQLPublishWithOpenSysMLInput }
  >(publishWithOpenSysMLMutation);
  const [answer, setAnswer] = useState<Answer | null>(null);

  const publish = (input: GQLPublishWithOpenSysMLInput): void => {
    performPublish({ variables: { input } })
      .then(({ data }) => {
        const payload = data?.publishWithOpenSysML;
        if (payload?.__typename === 'PublishWithOpenSysMLSuccessPayload') {
          setAnswer({ input, result: payload.result });
          return;
        }
        setAnswer(null);
        (payload?.messages ?? []).forEach((message) => addErrorMessage(message.body));
      })
      .catch(() => {
        setAnswer(null);
        addErrorMessage('An unexpected error has occurred, please refresh the page');
      });
  };

  return { publish, loading, input: answer?.input ?? null, result: answer?.result ?? null };
};
