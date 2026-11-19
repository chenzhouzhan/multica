import { z } from "zod";
import { InfrastructureImagePlanSchema } from "./images-schema";

const text = z.string().catch("");
const number = z.number().finite().catch(0);
const state = z.string().catch("UNKNOWN");
export const InfrastructureGrantSchema = z.object({
  cluster_id: z.number().positive(),
  service_id: z.number().positive(),
  role_id: z.number().positive(),
});
export const InfrastructureConnectionSchema = z.object({
  id: z.string().min(1),
  name: text,
  url: text,
  username: text,
  ca_pem: text,
  skip_tls_verify: z.boolean().catch(false),
  restart_grants: z.array(InfrastructureGrantSchema).catch([]),
});
export const InfrastructureConnectionsSchema = z.object({
  connections: z.array(InfrastructureConnectionSchema).catch([]),
  configured: z.boolean().catch(false),
  can_manage: z.boolean().catch(false),
});
export const InfrastructureClusterSchema = z.object({
  id: z.number().positive(),
  name: text,
});
export const InfrastructureNodeSchema = z.object({
  id: z.number().positive(),
  name: text,
  ip: text,
  cluster_id: number,
  cluster_name: text,
  status: state,
  cores: number,
  memory: number,
  disk: number,
});
export const InfrastructureProductSchema = z.object({
  type: text,
  name: text,
  version: text,
  display_version: text,
});
export const InfrastructureServiceSchema = z.object({
  id: z.number().positive(),
  name: text,
  type: text,
  type_name: text,
  version: text,
  product: InfrastructureProductSchema.nullable().catch(null),
  dependencies: z.array(z.number().positive()).catch([]),
  external_dependencies: z.array(z.string()).catch([]),
  cluster_name: text,
  cluster_id: number,
  health: state,
  busy: z.boolean().catch(false),
  installed: z.boolean().catch(false),
});
export const InfrastructureServiceRelationsSchema = z.object({
  dependencies: z.array(InfrastructureServiceSchema),
  dependents: z.array(InfrastructureServiceSchema),
});
export type InfrastructureService = z.infer<typeof InfrastructureServiceSchema>;
export const InfrastructureRoleSchema = z.object({
  id: z.number().positive(),
  name: text,
  type: text,
  service_id: number,
  service_name: text,
  node: InfrastructureNodeSchema,
  health: state,
  status: state,
  installed: z.boolean().catch(false),
});
export const InfrastructureLogSchema = z.object({
  hostname: text,
  service_id: number,
  service_name: text,
  time: text,
  level: state,
  message: text,
});
export const InfrastructureOperationSchema = z.object({
  id: z.string().min(1),
  connection_id: z.string().min(1),
  cluster_id: number,
  service_id: number,
  role_id: number,
  actor_type: text,
  actor_id: text,
  reason: text,
  job_id: z.number().nullable().catch(null),
  status: z
    .enum([
      "submitting",
      "submitted",
      "running",
      "succeeded",
      "failed",
      "verification_required",
      "unknown",
      "preparing",
      "ready",
      "expired",
      "conflict",
    ])
    .catch("unknown"),
  error: text,
  created_at: text,
  kind: z
    .enum(["restart_role", "publish_images", "restart_pods", "unknown"])
    .default("restart_role")
    .catch("unknown"),
  details: InfrastructureImagePlanSchema.nullable().catch(null),
});
export type InfrastructureConnections = z.infer<
  typeof InfrastructureConnectionsSchema
>;
export type InfrastructureConnection = z.infer<
  typeof InfrastructureConnectionSchema
>;
export type InfrastructureGrant = z.infer<typeof InfrastructureGrantSchema>;
export type InfrastructureRole = z.infer<typeof InfrastructureRoleSchema>;
export type InfrastructureOperation = z.infer<
  typeof InfrastructureOperationSchema
>;
export interface InfrastructureConnectionInput {
  name: string;
  url: string;
  username: string;
  password: string;
  ca_pem: string;
  skip_tls_verify: boolean;
}
export interface InfrastructureInput {
  connection_id: string;
  cluster_id?: number;
  service_id?: number;
  node_id?: number;
  role_id?: number;
  hostname?: string;
  level?: string;
  keywords?: string;
  start?: number;
  end?: number;
  reason?: string;
  operation_id?: string;
}
export type InfrastructureTool =
  | "list_clusters"
  | "list_nodes"
  | "list_services"
  | "service_relations"
  | "list_roles"
  | "search_logs"
  | "restart_role"
  | "get_operation"
  | "list_operations";

export const InfrastructureMetaEntrySchema = z.object({
  key: z.string().min(1),
  name: z.string(),
  type: z.string(),
});
export const InfrastructureMetaFileSchema = z.object({
  key: z.string().min(1),
  text: z.boolean(),
  content: z.string(),
});
export const InfrastructureMetaSavedSchema = z.object({
  saved: z.literal(true),
});
export type InfrastructureProduct = z.infer<typeof InfrastructureProductSchema>;
export type InfrastructureMetaEntry = z.infer<
  typeof InfrastructureMetaEntrySchema
>;
export type InfrastructureMetaFile = z.infer<
  typeof InfrastructureMetaFileSchema
>;
