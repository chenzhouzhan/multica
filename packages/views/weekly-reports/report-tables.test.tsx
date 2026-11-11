import { useState } from "react";
import { fireEvent, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { WeeklyTask } from "@multica/core/weekly-reports";
import { renderWithI18n } from "../test/i18n";
import { ProgressTables, TaskTable } from "./report-tables";

const task: WeeklyTask = {
  id: "task",
  version: "3.0",
  title: "API",
  completed: "Ready",
  spent: null,
  remaining: 0,
  note: "",
};
function Editor() {
  const [tasks, change] = useState([task]);
  return (
    <>
      <TaskTable tasks={tasks} onChange={change} />
      <output data-testid="content">{JSON.stringify(tasks)}</output>
    </>
  );
}
describe("weekly report tables", () => {
  it("preserves zero and blank workload through editing and supports row removal", () => {
    renderWithI18n(<Editor />);
    fireEvent.change(screen.getByLabelText("1: Effort this week"), {
      target: { value: "1.5" },
    });
    expect(
      JSON.parse(screen.getByTestId("content").textContent!)[0],
    ).toMatchObject({ spent: 1.5, remaining: 0 });
    fireEvent.change(screen.getByLabelText("1: Effort this week"), {
      target: { value: "" },
    });
    expect(
      JSON.parse(screen.getByTestId("content").textContent!)[0].spent,
    ).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Remove task 1" }));
    expect(screen.getByTestId("content")).toHaveTextContent("[]");
  });

  it("aggregates submitted rows with their member without merging equal task IDs", () => {
    renderWithI18n(
      <TaskTable
        tasks={[]}
        collection={{
          roster: [],
          missing: [],
          reports: ["Alice", "Bob"].map((name) => ({
            member: { userId: name, name, avatarUrl: "" },
            content: { tasks: [task], plan: "" },
            publishedAt: "",
          })),
        }}
      />,
    );
    const rows = screen.getAllByRole("row");
    expect(rows).toHaveLength(3);
    expect(within(rows[1]!).getByRole("cell", { name: "Alice" })).toBeVisible();
    expect(within(rows[2]!).getByRole("cell", { name: "Bob" })).toBeVisible();
  });

  it("keeps version and onsite progress in the five Excel columns with free text", () => {
    renderWithI18n(
      <ProgressTables
        overview={{
          versionName: "Logits",
          versionRows: [
            {
              id: "row",
              title: "API",
              type: "Feature",
              lastProgress: "Connector ready; app pending",
              thisPlan: "Verify",
              note: "",
            },
          ],
          onsiteRows: [],
        }}
      />,
    );
    expect(screen.getAllByRole("columnheader")).toHaveLength(10);
    expect(screen.getByText("Connector ready; app pending")).toBeVisible();
  });
});
