"use client";

import { useEffect, useId, useState } from "react";
import {
  buildMetainfoTree,
  type InfrastructureMetaEntry,
  type MetainfoTreeNode,
} from "@multica/core/infrastructure";
import { Button } from "@multica/ui/components/ui/button";
import {
  ChevronDown,
  ChevronRight,
  File,
  Folder,
  FolderOpen,
} from "@multica/ui/icons";
import { cn } from "@multica/ui/lib/utils";

interface Props {
  files: InfrastructureMetaEntry[];
  selectedKey: string;
  disabled: boolean;
  onSelect: (file: InfrastructureMetaEntry) => void;
}

export function MetainfoFileTree(props: Props) {
  const selectedPath =
    props.files.find((file) => file.key === props.selectedKey)?.name ?? "";
  return (
    <ul className="grid gap-0.5">
      {buildMetainfoTree(props.files).map((node) => (
        <TreeNode
          key={node.path}
          node={node}
          selectedPath={selectedPath}
          {...props}
        />
      ))}
    </ul>
  );
}

function TreeNode({
  node,
  selectedPath,
  ...props
}: Props & { node: MetainfoTreeNode; selectedPath: string }) {
  if (node.kind === "directory")
    return <Directory node={node} selectedPath={selectedPath} {...props} />;
  const selected = node.file.key === props.selectedKey;
  return (
    <li className="min-w-0">
      <Button
        variant="ghost"
        aria-label={node.path}
        aria-current={selected ? "page" : undefined}
        disabled={props.disabled && !selected}
        title={node.path}
        onClick={() => props.onSelect(node.file)}
        className={cn(
          "h-8 w-full min-w-0 justify-start gap-2 px-2 text-left",
          selected &&
            "bg-surface-selected text-surface-selected-foreground hover:bg-surface-selected",
        )}
      >
        <File className="size-4 shrink-0" />
        <span className="truncate">{node.name}</span>
      </Button>
    </li>
  );
}

function Directory({
  node,
  selectedPath,
  ...props
}: Props & {
  node: Extract<MetainfoTreeNode, { kind: "directory" }>;
  selectedPath: string;
}) {
  const containsSelection = selectedPath.startsWith(`${node.path}/`);
  const [expanded, setExpanded] = useState(containsSelection);
  const id = useId();
  useEffect(() => {
    if (containsSelection) setExpanded(true);
  }, [containsSelection, selectedPath]);
  const Chevron = expanded ? ChevronDown : ChevronRight;
  const FolderIcon = expanded ? FolderOpen : Folder;
  return (
    <li className="min-w-0">
      <Button
        variant="ghost"
        aria-label={node.path}
        aria-expanded={expanded}
        aria-controls={id}
        title={node.path}
        onClick={() => setExpanded(!expanded)}
        onKeyDown={(event) => {
          if (event.key === "ArrowRight" || event.key === "ArrowLeft") {
            event.preventDefault();
            setExpanded(event.key === "ArrowRight");
          }
        }}
        className={cn(
          "h-8 w-full min-w-0 justify-start gap-1.5 px-2 text-left",
          containsSelection ? "font-medium" : "text-muted-foreground",
        )}
      >
        <Chevron className="size-3 shrink-0" />
        <FolderIcon className="size-4 shrink-0" />
        <span className="truncate">{node.name}</span>
      </Button>
      {expanded && (
        <ul
          id={id}
          className="ml-3 grid gap-0.5 border-l border-border/60 pl-2"
        >
          {node.children.map((child) => (
            <TreeNode
              key={child.path}
              node={child}
              selectedPath={selectedPath}
              {...props}
            />
          ))}
        </ul>
      )}
    </li>
  );
}
