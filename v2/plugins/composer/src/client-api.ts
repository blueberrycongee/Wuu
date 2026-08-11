export interface ComposerCommandSurface {
  draft: string;
  running: boolean;
  setDraft(value: string): void;
  focus(): void;
  input(): HTMLTextAreaElement | null;
}
