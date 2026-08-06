import type { PluginGenerationApi } from "@wuu/plugin-sdk";

export function activate(api: PluginGenerationApi): void {
  api.registerSlot("conversation.header", {
    id: "developer-loop-status",
    render() {
      return api.react.createElement("span", null, `Plugin generation ${api.generation}`);
    },
  });
}
