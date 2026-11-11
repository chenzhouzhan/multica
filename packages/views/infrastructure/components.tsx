"use client";
import type { ReactNode } from "react";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@multica/ui/components/ui/select";
export function ErrorMessage({ error }: { error: unknown }) {
  return error ? (
    <p role="alert" className="text-body text-destructive break-words">
      {error instanceof Error ? error.message : String(error)}
    </p>
  ) : null;
}
export function Field({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <label className="grid min-w-0 gap-1.5 text-caption text-muted-foreground">
      {label}
      {children}
    </label>
  );
}
export function Picker({
  label,
  value,
  items,
  onChange,
}: {
  label: string;
  value: string;
  items: { value: string; label: string }[];
  onChange: (value: string) => void;
}) {
  return (
    <div className="grid min-w-0 gap-1.5">
      <span className="text-caption text-muted-foreground">{label}</span>
      <Select
        value={value || null}
        items={items}
        onValueChange={(next) => onChange(next ?? "")}
      >
        <SelectTrigger aria-label={label} className="w-full min-w-40">
          <SelectValue placeholder={label} />
        </SelectTrigger>
        <SelectContent>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
export function ResourceTable({
  headings,
  rows,
  empty,
}: {
  headings: string[];
  rows: ReactNode[][];
  empty: string;
}) {
  return (
    <div className="overflow-x-auto rounded-lg border">
      <table className="w-full text-left text-body">
        <thead className="bg-muted/40 text-caption text-muted-foreground">
          <tr>
            {headings.map((label, i) => (
              <th key={i} className="whitespace-nowrap px-4 py-3 font-medium">
                {label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((cells, i) => (
            <tr key={i} className="border-t">
              {cells.map((cell, j) => (
                <td
                  key={j}
                  className="max-w-80 break-words px-4 py-3 align-top"
                >
                  {cell}
                </td>
              ))}
            </tr>
          ))}
          {rows.length === 0 && (
            <tr>
              <td
                className="px-4 py-6 text-muted-foreground"
                colSpan={headings.length}
              >
                {empty}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
