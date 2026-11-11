export function weekOf(date: Date): string {
  const day = new Date(
    Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()),
  );
  day.setUTCDate(day.getUTCDate() - ((day.getUTCDay() + 6) % 7));
  return day.toISOString().slice(0, 10);
}
export function shiftWeek(week: string, amount: number): string {
  const day = new Date(`${week}T00:00:00Z`);
  day.setUTCDate(day.getUTCDate() + amount * 7);
  return day.toISOString().slice(0, 10);
}
export function validWeek(value: string | null): value is string {
  if (!value || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const day = new Date(`${value}T00:00:00Z`);
  return (
    Number.isFinite(day.getTime()) &&
    day.getUTCFullYear() >= 1970 &&
    day.getUTCDay() === 1 &&
    day.toISOString().slice(0, 10) === value
  );
}
export function weekRange(week: string): string {
  const end = new Date(`${week}T00:00:00Z`);
  end.setUTCDate(end.getUTCDate() + 6);
  return `${week} — ${end.toISOString().slice(0, 10)}`;
}
export function weeklyRowId(): string {
  const bytes = new Uint32Array(4);
  globalThis.crypto.getRandomValues(bytes);
  return Array.from(bytes, (value) => value.toString(16).padStart(8, "0")).join(
    "",
  );
}
