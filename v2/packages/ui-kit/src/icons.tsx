import { Blocks, FolderPlus, MessageSquarePlus, Search, Settings, type LucideIcon } from "lucide-react";

export interface WuuIconProps {
  size?: number;
  className?: string;
  "aria-hidden"?: boolean | "true" | "false";
  "aria-label"?: string;
}

function icon(Icon: LucideIcon) {
  return function WuuIcon({ size = 18, "aria-hidden": ariaHidden = true, ...props }: WuuIconProps) {
    return <Icon {...props} size={size} strokeWidth={1.8} color="currentColor" aria-hidden={ariaHidden} />;
  };
}

export const NewConversationIcon = icon(MessageSquarePlus);
export const SearchIcon = icon(Search);
export const PluginsIcon = icon(Blocks);
export const AddWorkspaceIcon = icon(FolderPlus);
export const SettingsIcon = icon(Settings);
