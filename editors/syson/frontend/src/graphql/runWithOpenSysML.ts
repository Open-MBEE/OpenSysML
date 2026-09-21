import { gql, useMutation } from '@apollo/client';
import { useMultiToast } from '@eclipse-sirius/sirius-components-core';
import { useEffect } from 'react';

interface GQLMessage {
  body: string;
  level: string;
}

interface GQLErrorPayload {
  __typename: 'ErrorPayload';
  id: string | null;
  messages: GQLMessage[] | null;
}

export type GQLRunOperation =
  | 'INSTANTIATE'
  | 'EXECUTE_ACTION'
  | 'EXPLORE_ACTION'
  | 'EXECUTE_STATE'
  | 'EXPLORE_STATE'
  | 'VERIFY_CONSTRAINT'
  | 'VERIFY_REQUIREMENT'
  | 'VERIFY_SATISFACTION'
  | 'EVALUATE_CALC'
  | 'RUN_ANALYSIS'
  | 'VALIDATE_INSTANCE';

export interface GQLRunInputValue {
  name: string;
  expression: string;
}

export interface GQLRunWithOpenSysMLInput {
  id: string;
  editingContextId: string;
  objectId: string;
  operation: GQLRunOperation;
  inputs: GQLRunInputValue[];
  events: string[];
  arguments: string[];
  schedule: string | null;
  subject: string | null;
}

export interface GQLOpenSysMLNamedValue {
  name: string;
  value: string;
}

export interface GQLOpenSysMLDiagnostic {
  severity: string;
  message: string;
  code: string;
  documentName: string | null;
  line: number | null;
  qualifiedName: string | null;
  elementId: string | null;
  siriusId: string | null;
}

export interface GQLOpenSysMLVerdict {
  subject: string;
  kind: string;
  holds: boolean;
  decided: boolean;
  detail: string | null;
  siriusId: string | null;
}

export interface GQLOpenSysMLInstance {
  id: number;
  type: string;
  typeSiriusId: string | null;
  featureValues: GQLOpenSysMLNamedValue[];
}

export interface GQLOpenSysMLRunOutcome {
  outputs: GQLOpenSysMLNamedValue[];
  finalState: string | null;
  trace: string[];
  error: string | null;
  linearizations: number;
  witness: string[];
}

export interface GQLOpenSysMLRunResult {
  modelHash: string;
  operation: GQLRunOperation;
  target: string;
  ok: boolean;
  verdict: string | null;
  schedule: string | null;
  finalTime: number | null;
  outputs: GQLOpenSysMLNamedValue[];
  trace: string[];
  outcomes: GQLOpenSysMLRunOutcome[];
  resultText: string | null;
  diagnostics: GQLOpenSysMLDiagnostic[];
  verdicts: GQLOpenSysMLVerdict[];
  instances: GQLOpenSysMLInstance[];
}

export interface GQLRunWithOpenSysMLSuccessPayload {
  __typename: 'RunWithOpenSysMLSuccessPayload';
  id: string;
  messages: GQLMessage[];
  result: GQLOpenSysMLRunResult;
}

export type GQLRunWithOpenSysMLPayload = GQLErrorPayload | GQLRunWithOpenSysMLSuccessPayload;

interface GQLRunWithOpenSysMLMutationData {
  runWithOpenSysML: GQLRunWithOpenSysMLPayload;
}

export const runWithOpenSysMLMutation = gql`
  mutation runWithOpenSysML($input: RunWithOpenSysMLInput!) {
    runWithOpenSysML(input: $input) {
      __typename
      ... on ErrorPayload {
        messages {
          body
          level
        }
      }
      ... on RunWithOpenSysMLSuccessPayload {
        id
        messages {
          body
          level
        }
        result {
          modelHash
          operation
          target
          ok
          verdict
          schedule
          finalTime
          outputs {
            name
            value
          }
          trace
          outcomes {
            outputs {
              name
              value
            }
            finalState
            trace
            error
            linearizations
            witness
          }
          resultText
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
          verdicts {
            subject
            kind
            holds
            decided
            detail
            siriusId
          }
          instances {
            id
            type
            typeSiriusId
            featureValues {
              name
              value
            }
          }
        }
      }
    }
  }
`;

export interface UseRunWithOpenSysMLValue {
  run: (input: GQLRunWithOpenSysMLInput) => void;
  loading: boolean;
  result: GQLOpenSysMLRunResult | null;
  messages: GQLMessage[];
}

export const useRunWithOpenSysML = (): UseRunWithOpenSysMLValue => {
  const { addErrorMessage, addMessages } = useMultiToast();
  const [performRun, { loading, data, error }] = useMutation<GQLRunWithOpenSysMLMutationData>(runWithOpenSysMLMutation);

  useEffect(() => {
    if (error) {
      addErrorMessage('An unexpected error has occurred, please refresh the page');
    }
    if (data) {
      const payload = data.runWithOpenSysML;
      if (payload.__typename === 'ErrorPayload' && payload.messages) {
        payload.messages.forEach((message) => addErrorMessage(message.body));
      }
    }
  }, [addErrorMessage, addMessages, data, error]);

  const run = (input: GQLRunWithOpenSysMLInput): void => {
    void performRun({ variables: { input } });
  };

  const payload = data?.runWithOpenSysML;
  return {
    run,
    loading,
    result: payload?.__typename === 'RunWithOpenSysMLSuccessPayload' ? payload.result : null,
    messages: payload?.messages ?? [],
  };
};
