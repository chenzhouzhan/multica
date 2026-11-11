import { create } from "zustand";
import { createDraftStore } from "../drafts/create-draft-store";

export const useMetainfoDraftStore = createDraftStore({
  storageKey: "multica_metainfo_drafts",
  emptyData: { files: {} as Record<string, string> },
  hasMeaningful: (data) => Object.keys(data.files).length > 0,
});

export function metainfoDraftKey(
  ws: string,
  manager: string,
  type: string,
  version: string,
  file: string,
) {
  return JSON.stringify([ws, manager, type, version, file]);
}

export function setMetainfoDraft(key: string, value: string | null) {
  const store = useMetainfoDraftStore.getState();
  const files = { ...store.draft.files };
  if (value === null) delete files[key];
  else files[key] = value;
  store.setDraft({ files });
}

type Guard = (proceed: () => void) => void;
export const useInfrastructureEditorStore = create<{
  guard: Guard | null;
  setGuard: (guard: Guard | null) => void;
}>((set) => ({ guard: null, setGuard: (guard) => set({ guard }) }));

export function requestInfrastructureNavigation(proceed: () => void) {
  const guard = useInfrastructureEditorStore.getState().guard;
  if (guard) guard(proceed);
  else proceed();
}
