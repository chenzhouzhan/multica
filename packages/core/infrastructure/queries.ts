import { api } from "../api";
import {
  InfrastructureClusterSchema,
  InfrastructureServiceSchema,
} from "./schema";

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
