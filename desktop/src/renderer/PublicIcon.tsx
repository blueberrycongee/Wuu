import {
  Activity,
  Archive,
  Bot,
  Brain,
  CalendarDays,
  Check,
  CircleCheck,
  Clock,
  Code2,
  Database,
  FileText,
  Folder,
  Gauge,
  Globe2,
  Inbox,
  LayoutGrid,
  ListTodo,
  MessageSquare,
  Moon,
  Plug,
  Search,
  Settings,
  Shield,
  SlidersHorizontal,
  Sparkles,
  Terminal,
  Users,
  Workflow,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import { isPublicIconName, type PublicIconName } from "../shared/themeContract.generated";
import { PluginBlocksIcon } from "./PluginBlocksIcon";

const PUBLIC_ICON_COMPONENTS = {
  archive: Archive,
  bot: Bot,
  brain: Brain,
  calendar: CalendarDays,
  check: Check,
  "check-circle": CircleCheck,
  clock: Clock,
  code: Code2,
  database: Database,
  "file-text": FileText,
  folder: Folder,
  gauge: Gauge,
  globe: Globe2,
  inbox: Inbox,
  "layout-grid": LayoutGrid,
  "list-todo": ListTodo,
  "message-square": MessageSquare,
  moon: Moon,
  plug: Plug,
  pulse: Activity,
  search: Search,
  settings: Settings,
  shield: Shield,
  sliders: SlidersHorizontal,
  sparkles: Sparkles,
  terminal: Terminal,
  users: Users,
  workflow: Workflow,
  wrench: Wrench,
} as const satisfies Readonly<Record<PublicIconName, LucideIcon>>;

export function PublicIcon({
  name,
  className,
}: {
  name?: string;
  className?: string;
}): JSX.Element {
  if (!name || !isPublicIconName(name)) {
    return <PluginBlocksIcon className={className} />;
  }
  const Icon = PUBLIC_ICON_COMPONENTS[name];
  return <Icon className={className} data-icon={name} />;
}
