import { Service, type Context } from "@wuu-v2/client-runtime";

export interface SlashCommandContext {
  client: Context;
  sessionId: string;
  args: string;
}

export type SlashCommandResult =
  | { type: "handled" }
  | { type: "replace"; draft: string }
  | { type: "submit"; text: string };

export interface SlashCommand {
  id: string;
  name: string;
  title: string;
  description?: string;
  aliases?: string[];
  keywords?: string[];
  order?: number;
  disabled?(context: Omit<SlashCommandContext, "args"> & { running: boolean }): string | undefined;
  execute(context: SlashCommandContext): SlashCommandResult | Promise<SlashCommandResult>;
}

function normalizeToken(value: string): string {
  const token = value.trim().toLowerCase();
  if (!token || token.startsWith("/") || /\s/.test(token)) {
    throw new Error(`invalid slash command token: ${value}`);
  }
  return token;
}

export class SlashCommandsService extends Service {
  private readonly commands = new Map<string, SlashCommand>();
  private readonly tokens = new Map<string, string>();
  private readonly listeners = new Set<() => void>();
  private revision = 0;

  constructor(ctx: Context) {
    super(ctx, "slashCommands");
  }

  register(command: SlashCommand): () => void {
    if (this.commands.has(command.id)) throw new Error(`duplicate slash command: ${command.id}`);
    const tokens = [command.name, ...(command.aliases ?? [])].map(normalizeToken);
    for (const token of tokens) {
      const owner = this.tokens.get(token);
      if (owner) throw new Error(`slash command token conflict: ${token} (${owner}, ${command.id})`);
    }
    const stored = { ...command, name: normalizeToken(command.name) };
    this.commands.set(command.id, stored);
    for (const token of tokens) this.tokens.set(token, command.id);
    this.changed();
    return this.ctx.effect(() => () => {
      if (!this.commands.delete(command.id)) return;
      for (const token of tokens) this.tokens.delete(token);
      this.changed();
    }, `unregister slash command:${command.id}`);
  }

  entries(): SlashCommand[] {
    return [...this.commands.values()].sort((left, right) =>
      (left.order ?? 0) - (right.order ?? 0) || left.id.localeCompare(right.id));
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  snapshot = (): number => this.revision;

  private changed(): void {
    this.revision += 1;
    for (const listener of this.listeners) listener();
  }
}

declare module "cordis" {
  interface Context {
    slashCommands: SlashCommandsService;
  }
}
