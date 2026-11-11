import { z } from "zod";

const text = z.string().catch("");
const workload = z.number().finite().nonnegative().nullable().catch(null);
export const WeeklyTaskSchema = z.object({
  id: z.string().min(1),
  version: text,
  title: text,
  completed: text,
  spent: workload,
  remaining: workload,
  note: text,
});
export const WeeklyContentSchema = z.object({
  tasks: z.array(WeeklyTaskSchema).catch([]),
  plan: text,
});
export const WeeklyProgressSchema = z
  .object({
    id: z.string().min(1),
    title: text,
    type: text,
    last_progress: text,
    this_plan: text,
    note: text,
  })
  .transform(({ last_progress, this_plan, ...row }) => ({
    ...row,
    lastProgress: last_progress,
    thisPlan: this_plan,
  }));
export const WeeklyOverviewSchema = z
  .object({
    version_name: text,
    version_rows: z.array(WeeklyProgressSchema).catch([]),
    onsite_rows: z.array(WeeklyProgressSchema).catch([]),
  })
  .transform((value) => ({
    versionName: value.version_name,
    versionRows: value.version_rows,
    onsiteRows: value.onsite_rows,
  }));
export const WeeklyMemberSchema = z
  .object({ user_id: z.string().min(1), name: text, avatar_url: text })
  .transform(({ user_id, avatar_url, name }) => ({
    userId: user_id,
    name,
    avatarUrl: avatar_url,
  }));
const ContributionSchema = z
  .object({
    member: WeeklyMemberSchema,
    content: WeeklyContentSchema,
    published_at: text,
  })
  .transform(({ published_at, ...value }) => ({
    ...value,
    publishedAt: published_at,
  }));
export const WeeklyCollectionSchema = z.object({
  roster: z.array(WeeklyMemberSchema).catch([]),
  reports: z.array(ContributionSchema).catch([]),
  missing: z.array(WeeklyMemberSchema).catch([]),
});
export const WeeklySnapshotSchema = z.object({
  overview: WeeklyOverviewSchema,
  collection: WeeklyCollectionSchema,
});
export const WeeklyReportSchema = z
  .object({
    id: text,
    user_id: z.string().min(1),
    author_name: text,
    week_start: z.string().min(1),
    revision: z.number().int().nonnegative(),
    draft: WeeklyContentSchema.nullish().catch(null),
    published: WeeklyContentSchema.nullish().catch(null),
    published_at: z.string().nullable().catch(null),
    updated_at: text,
    updated_by_type: text,
    updated_by_id: text,
  })
  .transform((value) => ({
    id: value.id,
    userId: value.user_id,
    authorName: value.author_name,
    weekStart: value.week_start,
    revision: value.revision,
    draft: value.draft ?? null,
    published: value.published ?? null,
    publishedAt: value.published_at,
    updatedAt: value.updated_at,
    updatedByType: value.updated_by_type,
    updatedById: value.updated_by_id,
  }));
export const WeeklyWeekSchema = z
  .object({
    week_start: z.string().min(1),
    roster: z.array(WeeklyMemberSchema).catch([]),
    reports: z.array(WeeklyReportSchema).catch([]),
  })
  .transform(({ week_start, ...value }) => ({
    ...value,
    weekStart: week_start,
  }));
export const WeeklyTeamSchema = z
  .object({
    id: text,
    week_start: z.string().min(1),
    revision: z.number().int().nonnegative(),
    draft: WeeklyOverviewSchema.nullish().catch(null),
    snapshot: WeeklySnapshotSchema.nullish().catch(null),
    generated_at: z.string().nullable().catch(null),
    can_manage: z.boolean().catch(false),
    collection: WeeklyCollectionSchema,
  })
  .transform((value) => ({
    id: value.id,
    weekStart: value.week_start,
    revision: value.revision,
    draft: value.draft ?? null,
    snapshot: value.snapshot ?? null,
    generatedAt: value.generated_at,
    canManage: value.can_manage,
    collection: value.collection,
  }));

export type WeeklyTask = z.infer<typeof WeeklyTaskSchema>;
export type WeeklyContent = z.infer<typeof WeeklyContentSchema>;
export type WeeklyProgress = z.infer<typeof WeeklyProgressSchema>;
export type WeeklyOverview = z.infer<typeof WeeklyOverviewSchema>;
export type WeeklyMember = z.infer<typeof WeeklyMemberSchema>;
export type WeeklyCollection = z.infer<typeof WeeklyCollectionSchema>;
export type WeeklySnapshot = z.infer<typeof WeeklySnapshotSchema>;
export type WeeklyReport = z.infer<typeof WeeklyReportSchema>;
export type WeeklyWeek = z.infer<typeof WeeklyWeekSchema>;
export type WeeklyTeam = z.infer<typeof WeeklyTeamSchema>;

export const emptyWeeklyContent = (): WeeklyContent => ({
  tasks: [],
  plan: "",
});
export const emptyWeeklyOverview = (): WeeklyOverview => ({
  versionName: "",
  versionRows: [],
  onsiteRows: [],
});
export function encodeWeeklyOverview(value: WeeklyOverview) {
  const rows = (items: WeeklyProgress[]) =>
    items.map(({ lastProgress, thisPlan, ...row }) => ({
      ...row,
      last_progress: lastProgress,
      this_plan: thisPlan,
    }));
  return {
    version_name: value.versionName,
    version_rows: rows(value.versionRows),
    onsite_rows: rows(value.onsiteRows),
  };
}
