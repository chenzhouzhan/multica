import { z } from "zod";

export const InfrastructureImageTargetSchema = z.object({
  key: z.string().min(1),
  role_type: z.string().catch(""),
  role_name: z.string().catch(""),
  kind: z.string().min(1),
  namespace: z.string().min(1),
  workload: z.string().min(1),
  uid: z.string().min(1),
  generation: z.number().int().positive(),
  container: z.string().min(1),
  image: z.string().min(1),
  replicas: z.number().int().nonnegative(),
  blocked_reason: z.string().catch(""),
});

export const InfrastructureImageResourcesSchema = z.object({
  configured: z.boolean(),
  targets: z.array(InfrastructureImageTargetSchema),
});

export const InfrastructureImagePlanSchema = z.object({
  items: z.array(
    z.object({
      target: InfrastructureImageTargetSchema,
      image: z.object({
        source: z.string(),
        pinned: z.string(),
        digest: z.string(),
        verification_digests: z.array(z.string()).catch([]),
      }),
      status: z
        .enum([
          "pending",
          "resolved",
          "submitting",
          "submitted",
          "succeeded",
          "failed",
          "skipped",
          "unknown",
          "conflict",
          "verification_required",
        ])
        .catch("unknown"),
      applied_generation: z.number().catch(0),
    }),
  ),
  steps: z.array(
    z.object({
      kind: z
        .enum([
          "discover",
          "resolve",
          "check",
          "update",
          "rollout",
          "verify",
          "unknown",
        ])
        .catch("unknown"),
      target_key: z.string().catch(""),
      status: z
        .enum([
          "pending",
          "running",
          "succeeded",
          "failed",
          "skipped",
          "unknown",
        ])
        .catch("unknown"),
      error: z.string().catch(""),
    }),
  ),
});

export const InfrastructureExecutionAccessSchema = z
  .object({
    configured: z.boolean(),
    host: z.string(),
    port: z.number().int().min(1).max(65535),
    username: z.string(),
    host_key: z.string(),
    context: z.string(),
    auth_type: z.enum(["key", "password"]),
    requires_host_key_confirmation: z.boolean().catch(false),
    host_key_fingerprint: z.string().catch(""),
    previous_host_key_fingerprint: z.string().catch(""),
    requires_context_selection: z.boolean().catch(false),
    context_options: z
      .array(z.object({ name: z.string().min(1), cluster: z.string() }))
      .catch([]),
  })
  .refine(
    (value) =>
      (!value.requires_context_selection ||
        (!value.configured && value.context_options.length > 1)) &&
      (!value.requires_host_key_confirmation ||
        (!value.configured &&
          !value.requires_context_selection &&
          value.host_key.length > 0 &&
          value.host_key_fingerprint.startsWith("SHA256:"))),
  );

export interface InfrastructureImageScope {
  ws: string;
  connection: string;
  cluster: number;
  service: number;
}
export interface InfrastructureExecutionAccessInput {
  host: string;
  port: number;
  username: string;
  host_key: string;
  context: string;
  auth_type: "key" | "password";
  private_key: string;
  password: string;
  probe_host_key?: boolean;
}
export interface InfrastructureImagePreviewInput {
  action: "publish_images" | "restart_pods";
  items: { key: string; source: string }[];
  reason: string;
}
export type InfrastructureImageTarget = z.infer<
  typeof InfrastructureImageTargetSchema
>;
export type InfrastructureImageResources = z.infer<
  typeof InfrastructureImageResourcesSchema
>;
export type InfrastructureImagePlan = z.infer<
  typeof InfrastructureImagePlanSchema
>;
export type InfrastructureExecutionAccess = z.infer<
  typeof InfrastructureExecutionAccessSchema
>;
