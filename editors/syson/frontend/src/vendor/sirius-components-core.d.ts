declare module '@eclipse-sirius/sirius-components-core' {
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
    FallbackComponent: import('react').ComponentType<P>;
  }

  export interface ComponentExtension<P> {
    identifier: string;
    Component: import('react').ComponentType<P>;
  }

  export class ExtensionRegistry {
    putData<P>(extensionPoint: DataExtensionPoint<P>, extension: DataExtension<P>): void;
    getData<P>(extensionPoint: DataExtensionPoint<P>): DataExtension<P> | null;
    addComponent<P>(extensionPoint: ComponentExtensionPoint<P>, extension: ComponentExtension<P>): void;
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

  export const SelectionContext: import('react').Context<SelectionContextValue>;
  export const useSelection: () => SelectionContextValue;

  export interface GQLMessage {
    body: string;
    level: string;
  }

  export interface GQLErrorPayload {
    __typename: 'ErrorPayload';
    id: string | null;
    messages: GQLMessage[] | null;
  }

  export type ToastVariant = 'error' | 'info' | 'warning' | 'success' | 'default';

  export interface MessageOptions {
    variant: ToastVariant;
  }

  export interface ToastContextValue {
    enqueueSnackbar: (body: string, options?: MessageOptions) => void;
  }

  export const ToastContext: import('react').Context<ToastContextValue>;
  export const useMultiToast: () => {
    addErrorMessage: (message: string) => void;
    addMessages: (messages: GQLMessage[]) => void[];
  };

  export interface GQLStyledStringFragment {
    text: string;
  }

  export interface GQLStyledString {
    styledStringFragments: GQLStyledStringFragment[];
  }
}
