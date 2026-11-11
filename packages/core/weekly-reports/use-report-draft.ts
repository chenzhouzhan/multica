import { useCallback, useEffect, useRef, useState } from "react";

export type DraftStatus = "saved" | "unsaved" | "saving" | "error";

// Serialize saves so edits made during a request use the returned revision.
export function useReportDraft<T>(
  initial: T,
  initialRevision: number,
  save: (content: T, revision: number) => Promise<number>,
) {
  const [content, setContent] = useState(initial);
  const [status, setStatus] = useState<DraftStatus>("saved");
  const [error, setError] = useState<unknown>(null);
  const latest = useRef(initial);
  const revision = useRef(initialRevision);
  const generation = useRef(0);
  const savedGeneration = useRef(0);
  const failed = useRef(false);
  const running = useRef<Promise<number> | null>(null);
  const saveRef = useRef(save);
  useEffect(() => {
    saveRef.current = save;
  }, [save]);

  const change = useCallback((next: T) => {
    failed.current = false;
    latest.current = next;
    generation.current += 1;
    setContent(next);
    setError(null);
    setStatus("unsaved");
  }, []);

  const flush = useCallback((): Promise<number> => {
    if (running.current) return running.current;
    if (savedGeneration.current === generation.current)
      return Promise.resolve(revision.current);
    const work = async () => {
      setStatus("saving");
      try {
        while (savedGeneration.current !== generation.current) {
          const current = generation.current;
          revision.current = await saveRef.current(
            latest.current,
            revision.current,
          );
          savedGeneration.current = current;
        }
        setStatus("saved");
        setError(null);
        return revision.current;
      } catch (reason) {
        failed.current = true;
        setStatus("error");
        setError(reason);
        throw reason;
      } finally {
        running.current = null;
      }
    };
    running.current = work();
    return running.current;
  }, []);

  useEffect(() => {
    if (status !== "unsaved") return;
    const timer = setTimeout(() => {
      void flush().catch(() => undefined);
    }, 600);
    return () => clearTimeout(timer);
  }, [content, status, flush]);
  useEffect(
    () => () => {
      if (!failed.current) void flush().catch(() => undefined);
    },
    [flush],
  );
  return { content, change, status, error, flush };
}
