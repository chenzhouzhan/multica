import type { InfrastructureMetaEntry } from "./schema";

export type MetainfoTreeNode =
  | {
      kind: "directory";
      name: string;
      path: string;
      children: MetainfoTreeNode[];
    }
  | { kind: "file"; name: string; path: string; file: InfrastructureMetaEntry };

export function buildMetainfoTree(
  files: InfrastructureMetaEntry[],
): MetainfoTreeNode[] {
  const root: MetainfoTreeNode[] = [];
  for (const file of files) {
    const segments = file.name.split("/");
    let siblings = root;
    for (let index = 0; index < segments.length; index++) {
      const name = segments[index]!;
      const path = segments.slice(0, index + 1).join("/");
      if (index === segments.length - 1) {
        siblings.push({ kind: "file", name, path, file });
      } else {
        let directory = siblings.find(
          (node) => node.kind === "directory" && node.path === path,
        );
        if (!directory) {
          directory = { kind: "directory", name, path, children: [] };
          siblings.push(directory);
        }
        if (directory.kind === "directory") siblings = directory.children;
      }
    }
  }
  function sort(nodes: MetainfoTreeNode[]) {
    nodes.sort((a, b) => {
      const mainA = a.kind === "file" && a.file.type === "INFO_YAML";
      const mainB = b.kind === "file" && b.file.type === "INFO_YAML";
      if (mainA !== mainB) return mainA ? -1 : 1;
      if (a.kind !== b.kind) return a.kind === "directory" ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
    for (const node of nodes)
      if (node.kind === "directory") sort(node.children);
  }
  sort(root);
  return root;
}
