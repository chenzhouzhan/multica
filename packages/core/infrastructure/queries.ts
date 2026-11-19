import { api } from "../api";
import {
  InfrastructureClusterSchema,
  InfrastructureServiceSchema,
  InfrastructureOperationSchema,
} from "./schema";
import type { InfrastructureImageScope } from "./images-schema";

export const infrastructureImagesKey = (scope: InfrastructureImageScope) => [
  ...infrastructureResourceKey(scope.ws, scope.connection),
  scope.cluster,
  scope.service,
  "images",
];
export const infrastructureImagesOptions = (
  scope: InfrastructureImageScope,
) => ({
  queryKey: infrastructureImagesKey(scope),
  queryFn: () => api.infrastructureImageResources(scope),
  enabled:
    !!scope.ws && !!scope.connection && scope.cluster > 0 && scope.service > 0,
});
export const infrastructureImageOperationOptions = (
  ws: string,
  connection: string,
  id: string,
) => ({
  queryKey: [...infrastructureResourceKey(ws, connection), "operation", id],
  queryFn: () =>
    api.infrastructureQuery(
      ws,
      "get_operation",
      { connection_id: connection, operation_id: id },
      InfrastructureOperationSchema,
    ),
  enabled: !!ws && !!connection && !!id,
});

export const infrastructureConnectionsKey = (ws: string) => [
  "infrastructure",
  ws,
  "connections",
];
export const infrastructureResourceKey = (ws: string, connection: string) => [
  "infrastructure",
  ws,
  connection,
];
export const infrastructureConnectionsOptions = (ws: string) => ({
  queryKey: infrastructureConnectionsKey(ws),
  queryFn: () => api.listInfrastructureConnections(ws),
  enabled: !!ws,
});
export const infrastructureClustersOptions = (
  ws: string,
  connection: string,
) => ({
  queryKey: [...infrastructureResourceKey(ws, connection), "clusters"],
  queryFn: () =>
    api.infrastructureQuery(
      ws,
      "list_clusters",
      { connection_id: connection },
      InfrastructureClusterSchema.array(),
    ),
  enabled: !!ws && !!connection,
});
export const infrastructureServicesOptions = (
  ws: string,
  connection: string,
  cluster: number,
) => ({
  queryKey: [...infrastructureResourceKey(ws, connection), cluster, "services"],
  queryFn: () =>
    api.infrastructureQuery(
      ws,
      "list_services",
      { connection_id: connection, cluster_id: cluster },
      InfrastructureServiceSchema.array(),
    ),
  enabled: !!ws && !!connection && cluster > 0,
});
