import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { EditorView } from "@codemirror/view";
import { undo, redo } from "@codemirror/commands";
import { CodeEditor } from "./code-editor";

function editorView(container: HTMLElement) {
  return EditorView.findFromDOM(
    container.querySelector<HTMLElement>(".cm-editor")!,
  )!;
}

describe("CodeEditor", () => {
  it("highlights YAML and properties and keeps the initial load out of undo history", () => {
    const onChange = vi.fn();
    const { container, rerender, unmount } = render(
      <CodeEditor
        aria-label="File"
        language="yaml"
        value={"name: studio\nenabled: true\n"}
        onChange={onChange}
      />,
    );
    const editor = editorView(container);
    expect(screen.getByRole("textbox", { name: "File" })).toHaveTextContent(
      "name: studio",
    );
    expect(container.querySelectorAll(".cm-line span").length).toBeGreaterThan(
      1,
    );
    expect(onChange).not.toHaveBeenCalled();
    let undone = false;
    act(() => {
      undone = undo(editor);
    });
    expect(undone).toBe(false);
    act(() => {
      editor.dispatch({ changes: { from: 6, to: 12, insert: "updated" } });
    });
    expect(onChange).toHaveBeenLastCalledWith("name: updated\nenabled: true\n");
    act(() => {
      undo(editor);
    });
    expect(onChange).toHaveBeenLastCalledWith("name: studio\nenabled: true\n");
    act(() => {
      redo(editor);
    });
    expect(onChange).toHaveBeenLastCalledWith("name: updated\nenabled: true\n");
    onChange.mockClear();
    rerender(
      <CodeEditor
        aria-label="File"
        language="properties"
        value={"# Settings\nservice.name=Studio\n"}
        onChange={onChange}
      />,
    );
    expect(editor.state.doc.toString()).toBe(
      "# Settings\nservice.name=Studio\n",
    );
    expect(
      container.querySelector('[data-code-language="properties"]'),
    ).toBeInTheDocument();
    expect(container.querySelectorAll(".cm-line span").length).toBeGreaterThan(
      1,
    );
    expect(onChange).not.toHaveBeenCalled();
    unmount();
    expect(container.querySelector(".cm-editor")).not.toBeInTheDocument();
  });

  it("switches read-only mode without replacing the document or edit history", () => {
    const onChange = vi.fn();
    const { container, rerender } = render(
      <CodeEditor
        aria-label="Config"
        language="yaml"
        value="key: value"
        onChange={onChange}
        readOnly
      />,
    );
    const editor = editorView(container);
    const content = screen.getByRole("textbox", { name: "Config" });
    expect(content).toHaveAttribute("aria-readonly", "true");
    expect(editor.state.readOnly).toBe(true);
    rerender(
      <CodeEditor
        aria-label="Config"
        language="yaml"
        value="key: value"
        onChange={onChange}
      />,
    );
    expect(editorView(container)).toBe(editor);
    expect(content).toHaveAttribute("aria-readonly", "false");
    expect(editor.state.readOnly).toBe(false);
    act(() => {
      editor.dispatch({ changes: { from: 10, insert: "2" } });
    });
    rerender(
      <CodeEditor
        aria-label="Config"
        language="yaml"
        value="key: value2"
        onChange={onChange}
        readOnly
      />,
    );
    expect(editor.state.doc.toString()).toBe("key: value2");
    expect(editor.state.readOnly).toBe(true);
  });
});
