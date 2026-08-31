import { Blocks, type LucideProps } from "lucide-react";

/** Shared plugin mark, kept as a semantic wrapper around the product icon set. */
export function PluginBlocksIcon(props: LucideProps): JSX.Element {
  return (
    <Blocks
      aria-hidden="true"
      data-icon="plugin-blocks"
      {...props}
    />
  );
}
