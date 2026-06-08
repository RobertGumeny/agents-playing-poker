// Markdown page tool definitions for the wiki agent: read-only page list/read used at decision
// time, plus the page-write tool registered only for the post-hand update session.

import { Type } from "typebox";
import { defineTool, type ToolDefinition } from "@earendil-works/pi-coding-agent";

import { listPages, readPage, writePage } from "./pages.js";

export type WikiDirProvider = () => string | null;

export function createReadTools(getWikiDir: WikiDirProvider): ToolDefinition[] {
  return [
    defineTool({
      name: "md_list_pages",
      label: "Wiki Pages",
      description: "List all page names in the opponent wiki (relative slugs such as villain, patterns/folds-to-cbet).",
      parameters: Type.Object({}),
      async execute() {
        const dir = getWikiDir();
        const pages = dir ? await listPages(dir) : [];
        return jsonResult({ pages });
      },
    }),
    defineTool({
      name: "md_read_page",
      label: "Wiki Read",
      description: "Return the raw markdown of a named wiki page. Follow a [[link]] by reading its target. Returns found:false for a missing page.",
      parameters: Type.Object({
        page: Type.String({ description: "Page slug, for example villain or patterns/folds-to-cbet" }),
      }),
      async execute(_toolCallId, params) {
        const dir = getWikiDir();
        if (!dir) return jsonResult({ found: false, page: params.page });
        return jsonResult(await readPage(dir, params.page));
      },
    }),
  ];
}

export function createWriteTool(getWikiDir: WikiDirProvider): ToolDefinition {
  return defineTool({
    name: "md_write_page",
    label: "Wiki Write",
    description: "Create or replace a wiki page with the given markdown content. Use [[links]] to connect pages. Does not delete pages.",
    parameters: Type.Object({
      page: Type.String({ description: "Page slug to create or overwrite, for example villain or patterns/folds-to-cbet" }),
      content: Type.String({ description: "Full markdown content for the page" }),
    }),
    async execute(_toolCallId, params) {
      const dir = getWikiDir();
      if (!dir) return jsonResult({ written: false, page: params.page, error: "no wiki directory available" });
      const written = await writePage(dir, params.page, params.content);
      return written
        ? jsonResult({ written: true, page: written })
        : jsonResult({ written: false, page: params.page, error: "invalid page name" });
    },
  });
}

function jsonResult(details: unknown) {
  return {
    content: [{ type: "text" as const, text: JSON.stringify(details, null, 2) }],
    details,
  };
}
