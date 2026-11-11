// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  metainfoDraftKey,
  requestInfrastructureNavigation,
  setMetainfoDraft,
  useInfrastructureEditorStore,
  useMetainfoDraftStore,
} from "./editor-store";

afterEach(() => {
  useInfrastructureEditorStore.getState().setGuard(null);
  useMetainfoDraftStore.getState().clearDraft();
});

describe("metainfo editing", () => {
  it("isolates drafts by workspace, Manager, service version and file", () => {
    const key = metainfoDraftKey(
      "ws-a",
      "manager-a",
      "LOGITS",
      "v1",
      "metainfo.yaml",
    );
    setMetainfoDraft(key, "");
    expect(useMetainfoDraftStore.getState().hasDraft()).toBe(true);
    expect(useMetainfoDraftStore.getState().draft.files[key]).toBe("");
    for (const other of [
      metainfoDraftKey("ws-b", "manager-a", "LOGITS", "v1", "metainfo.yaml"),
      metainfoDraftKey("ws-a", "manager-b", "LOGITS", "v1", "metainfo.yaml"),
      metainfoDraftKey("ws-a", "manager-a", "LOGITS", "v2", "metainfo.yaml"),
      metainfoDraftKey(
        "ws-a",
        "manager-a",
        "LOGITS",
        "v1",
        "configuration.yaml",
      ),
    ])
      expect(
        useMetainfoDraftStore.getState().draft.files[other],
      ).toBeUndefined();
    setMetainfoDraft(key, null);
    expect(useMetainfoDraftStore.getState().hasDraft()).toBe(false);
  });
  it("defers navigation until the editor permits leaving", () => {
    const navigate = vi.fn();
    let pending: (() => void) | undefined;
    useInfrastructureEditorStore.getState().setGuard((proceed) => {
      pending = proceed;
    });
    requestInfrastructureNavigation(navigate);
    expect(navigate).not.toHaveBeenCalled();
    useInfrastructureEditorStore.getState().setGuard(null);
    pending?.();
    expect(navigate).toHaveBeenCalledOnce();
    requestInfrastructureNavigation(navigate);
    expect(navigate).toHaveBeenCalledTimes(2);
  });
});
