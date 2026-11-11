"use client";

import { useEffect, useRef, useState } from "react";
import { basicSetup } from "codemirror";
import {
  Annotation,
  Compartment,
  EditorState,
  Transaction,
} from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { indentWithTab } from "@codemirror/commands";
import {
  HighlightStyle,
  StreamLanguage,
  syntaxHighlighting,
} from "@codemirror/language";
import { yaml } from "@codemirror/lang-yaml";
import { properties } from "@codemirror/legacy-modes/mode/properties";
import { tags } from "@lezer/highlight";
import { cn } from "@multica/ui/lib/utils";

export type CodeEditorLanguage = "yaml" | "properties" | "text";

const externalUpdate = Annotation.define<boolean>();
const highlightStyle = HighlightStyle.define([
  {
    tag: [tags.propertyName, tags.definition(tags.variableName)],
    color: "var(--info)",
  },
  { tag: [tags.string, tags.quote], color: "var(--success)" },
  {
    tag: [tags.keyword, tags.bool, tags.null, tags.number],
    color: "var(--brand)",
  },
  { tag: tags.comment, color: "var(--muted-foreground)", fontStyle: "italic" },
  { tag: tags.meta, color: "var(--info)" },
]);
const theme = EditorView.theme({
  "&": {
    height: "100%",
    fontSize: "var(--text-body)",
    color: "var(--foreground)",
    backgroundColor: "var(--background)",
  },
  "&.cm-focused": { outline: "none" },
  ".cm-scroller": {
    overflow: "auto",
    fontFamily: "var(--font-mono, ui-monospace, monospace)",
    lineHeight: "var(--text-body--line-height)",
    fontVariantLigatures: "none",
  },
  ".cm-content": { padding: "8px 0", caretColor: "var(--foreground)" },
  ".cm-cursor": { borderLeftColor: "var(--foreground)" },
  ".cm-gutters": {
    color: "var(--muted-foreground)",
    backgroundColor: "var(--muted)",
    borderRight: "1px solid var(--border)",
  },
  ".cm-activeLine, .cm-activeLineGutter": {
    backgroundColor: "var(--surface-hover)",
  },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, .cm-content ::selection":
    { backgroundColor: "var(--surface-selected)" },
  ".cm-panels": {
    color: "var(--foreground)",
    backgroundColor: "var(--background)",
  },
  ".cm-textfield, .cm-button": {
    color: "var(--foreground)",
    background: "var(--muted)",
    border: "1px solid var(--border)",
  },
});

export interface CodeEditorProps {
  value: string;
  onChange: (value: string) => void;
  language: CodeEditorLanguage;
  readOnly?: boolean;
  "aria-label": string;
  className?: string;
}

export function CodeEditor({
  value,
  onChange,
  language,
  readOnly = false,
  "aria-label": label,
  className,
}: CodeEditorProps) {
  const parent = useRef<HTMLDivElement>(null);
  const view = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  const [languageConfig] = useState(() => new Compartment());
  const [accessConfig] = useState(() => new Compartment());

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);
  useEffect(() => {
    const editor = new EditorView({
      parent: parent.current!,
      state: EditorState.create({
        extensions: [
          basicSetup,
          keymap.of([indentWithTab]),
          EditorState.tabSize.of(2),
          theme,
          syntaxHighlighting(highlightStyle),
          languageConfig.of([]),
          accessConfig.of([]),
          EditorView.updateListener.of((update) => {
            if (
              update.docChanged &&
              !update.transactions.some((transaction) =>
                transaction.annotation(externalUpdate),
              )
            ) {
              onChangeRef.current(update.state.doc.toString());
            }
          }),
        ],
      }),
    });
    view.current = editor;
    return () => {
      view.current = null;
      editor.destroy();
    };
  }, [languageConfig, accessConfig]);

  useEffect(() => {
    const editor = view.current;
    if (editor && editor.state.doc.toString() !== value) {
      editor.dispatch({
        changes: { from: 0, to: editor.state.doc.length, insert: value },
        annotations: [
          externalUpdate.of(true),
          Transaction.addToHistory.of(false),
        ],
      });
    }
  }, [value]);
  useEffect(() => {
    view.current?.dispatch({
      effects: languageConfig.reconfigure(
        language === "yaml"
          ? yaml()
          : language === "properties"
            ? StreamLanguage.define(properties)
            : [],
      ),
    });
  }, [language, languageConfig]);
  useEffect(() => {
    view.current?.dispatch({
      effects: accessConfig.reconfigure([
        EditorState.readOnly.of(readOnly),
        EditorView.editable.of(!readOnly),
        EditorView.contentAttributes.of({
          "aria-label": label,
          "aria-readonly": String(readOnly),
          tabindex: "0",
        }),
      ]),
    });
  }, [readOnly, label, accessConfig]);

  return (
    <div
      ref={parent}
      data-code-language={language}
      className={cn(
        "h-80 min-w-0 overflow-hidden rounded-lg border border-input focus-within:ring-2 focus-within:ring-ring",
        className,
      )}
    />
  );
}
