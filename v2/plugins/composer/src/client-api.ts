import type { JsonValue } from "@wuu-v2/contracts";
import type { ReactNode } from "react";

export interface ComposerCommandSurface {
  draft: string;
  running: boolean;
  context?: JsonValue;
  setDraft(value: string): void;
  submit(text: string): Promise<void>;
  focus(): void;
  input(): HTMLTextAreaElement | null;
}

export interface ComposerSurfaceProps {
  sessionId: string;
  draft: string;
  running: boolean;
  ariaLabel: string;
  autoFocus?: boolean;
  placeholder?: string;
  commands?: boolean;
  commandContext?: JsonValue;
  footer?: ReactNode;
  onVisualHeightChange?(height: number): void;
  onDraftChange(value: string): void;
  onSubmit(text: string): Promise<void> | void;
  onCancel?(): Promise<void> | void;
}

export interface ComposerSurfaceRenderer {
  render(props: ComposerSurfaceProps): ReactNode;
}
