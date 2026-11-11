// @vitest-environment jsdom
import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useReportDraft } from "./use-report-draft";

describe("weekly report autosave", () => {
  it("serializes edits made during a save using the returned revision", async () => {
    let finish!: (revision: number) => void;
    const save = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise<number>((resolve) => {
            finish = resolve;
          }),
      )
      .mockResolvedValueOnce(3);
    const { result } = renderHook(() =>
      useReportDraft<string>("initial", 1, save),
    );
    let pending!: Promise<number>;
    act(() => {
      result.current.change("first");
      pending = result.current.flush();
    });
    act(() => result.current.change("second"));
    await act(async () => {
      finish(2);
      await pending;
    });
    expect(save.mock.calls).toEqual([
      ["first", 1],
      ["second", 2],
    ]);
    expect(result.current.content).toBe("second");
    expect(result.current.status).toBe("saved");
  });
  it("keeps input and reports a conflict instead of retrying a stale document", async () => {
    const conflict = new Error("revision conflict");
    const save = vi.fn().mockRejectedValue(conflict);
    const { result } = renderHook(() =>
      useReportDraft<string>("initial", 4, save),
    );
    act(() => result.current.change("local edit"));
    await act(async () => {
      await expect(result.current.flush()).rejects.toBe(conflict);
    });
    await waitFor(() => expect(result.current.status).toBe("error"));
    expect(result.current.content).toBe("local edit");
    expect(save).toHaveBeenCalledTimes(1);
  });
});
