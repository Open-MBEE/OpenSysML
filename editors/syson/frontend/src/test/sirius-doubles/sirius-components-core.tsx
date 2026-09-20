import React, { useContext } from 'react';

export interface DataExtensionPoint<P> {
  identifier: string;
  fallback: P;
}

export interface DataExtension<P> {
  identifier: string;
  data: P;
}

export interface ComponentExtensionPoint<P> {
  identifier: string;
  FallbackComponent: React.ComponentType<P>;
}

export interface ComponentExtension<P> {
  identifier: string;
  Component: React.ComponentType<P>;
}

export class ExtensionRegistry {
  private readonly data: Record<string, DataExtension<unknown>> = {};
  private readonly components: Record<string, ComponentExtension<unknown>[]> = {};

  public putData<P>(extensionPoint: DataExtensionPoint<P>, extension: DataExtension<P>): void {
    this.data[extensionPoint.identifier] = extension as DataExtension<unknown>;
  }

  public getData<P>(extensionPoint: DataExtensionPoint<P>): DataExtension<P> | null {
    return (this.data[extensionPoint.identifier] as DataExtension<P> | undefined) ?? null;
  }

  public addComponent<P>(extensionPoint: ComponentExtensionPoint<P>, extension: ComponentExtension<P>): void {
    const components = this.components[extensionPoint.identifier] ?? [];
    components.push(extension as ComponentExtension<unknown>);
    this.components[extensionPoint.identifier] = components;
  }
}

export interface SelectionEntry {
  id: string;
}

export interface Selection {
  entries: SelectionEntry[];
}

export interface SelectionContextValue {
  selection: Selection;
  setSelection: (selection: Selection) => void;
}

const defaultSelection: SelectionContextValue = {
  selection: { entries: [] },
  setSelection: () => {},
};

export const SelectionContext = React.createContext<SelectionContextValue>(defaultSelection);

export const useSelection = (): SelectionContextValue => useContext(SelectionContext);

export interface GQLMessage {
  body: string;
  level: string;
}

export interface GQLErrorPayload {
  __typename: 'ErrorPayload';
  id: string | null;
  messages: GQLMessage[] | null;
}

type ToastVariant = 'error' | 'info' | 'warning' | 'success' | 'default';

export interface MessageOptions {
  variant: ToastVariant;
}

export interface ToastContextValue {
  enqueueSnackbar: (body: string, options?: MessageOptions) => void;
}

export const ToastContext = React.createContext<ToastContextValue>({
  enqueueSnackbar: () => {},
});

const getVariantFromMessageLevel = (level: string): ToastVariant => {
  switch (level) {
    case 'ERROR':
    case 'error':
      return 'error';
    case 'INFO':
    case 'info':
      return 'info';
    case 'WARNING':
    case 'warning':
      return 'warning';
    case 'SUCCESS':
    case 'success':
      return 'success';
    default:
      return 'default';
  }
};

export const useMultiToast = () => {
  const { enqueueSnackbar } = useContext(ToastContext);
  const addMessages = (messages: GQLMessage[]) =>
    messages.map((message) => enqueueSnackbar(message.body, { variant: getVariantFromMessageLevel(message.level) }));
  const addErrorMessage = (message: string) => addMessages([{ body: message, level: 'error' }]);
  return { addErrorMessage, addMessages };
};

export interface GQLStyledStringFragment {
  text: string;
}

export interface GQLStyledString {
  styledStringFragments: GQLStyledStringFragment[];
}
