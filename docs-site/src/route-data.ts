import { defineRouteMiddleware } from "@astrojs/starlight/route-data"

export const onRequest = defineRouteMiddleware((context) => {
  const route = context.locals.starlightRoute
  const marker = `__locale:${route.locale ?? "zh-cn"}`
  const localeSidebar = route.sidebar.find(
    (entry) => entry.type === "group" && entry.label === marker,
  )

  if (localeSidebar?.type === "group") {
    route.sidebar = localeSidebar.entries
    route.hasSidebar = localeSidebar.entries.length > 0
  }

  const sourcePath = route.entry.filePath?.split("generated-content/docs/").at(-1)
  if (sourcePath) {
    route.editUrl = new URL(
      sourcePath,
      "https://github.com/blueberrycongee/wuu/edit/main/docs/",
    )
  }
})
