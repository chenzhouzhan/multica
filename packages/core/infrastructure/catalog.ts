import type { InfrastructureService } from "./schema";

export function clusterHref(
  base: string,
  connection: string,
  cluster: number,
  service?: number,
) {
  const search = new URLSearchParams({
    manager: connection,
    cluster: String(cluster),
  });
  if (service !== undefined) search.set("service", String(service));
  return `${base}?${search}`;
}

export function infrastructureHref(
  base: string,
  current: URLSearchParams,
  patch: Record<string, string | number | null>,
) {
  const search = new URLSearchParams(current);
  for (const [key, value] of Object.entries(patch)) {
    if (value === null || value === "") search.delete(key);
    else search.set(key, String(value));
  }
  return `${base}?${search}`;
}

export function filterInfrastructureServices(
  services: InfrastructureService[],
  filters: { search: string; health: string; product: string },
) {
  const search = filters.search.trim().toLocaleLowerCase();
  return services.filter((service) => {
    const productKey =
      service.product?.type && service.product.type !== "__FALLBACK__"
        ? JSON.stringify([service.product.type])
        : "other";
    return (
      (!search ||
        [
          service.name,
          service.type,
          service.type_name,
          service.version,
          service.product?.name,
          service.product?.version,
          service.product?.display_version,
        ].some((value) => value?.toLocaleLowerCase().includes(search))) &&
      (filters.health !== "unhealthy" || service.health !== "HEALTHY") &&
      (!filters.product || filters.product === productKey)
    );
  });
}

export interface InfrastructureProductGroup {
  key: string;
  product: InfrastructureService["product"];
  services: InfrastructureService[];
  versions: InfrastructureProductVersion[];
}

export interface InfrastructureProductVersion {
  key: string;
  product: InfrastructureService["product"];
  services: InfrastructureService[];
}

export function groupInfrastructureProducts(
  services: InfrastructureService[],
): InfrastructureProductGroup[] {
  const groups = new Map<string, InfrastructureProductGroup>();
  for (const service of services) {
    const product =
      service.product?.type && service.product.type !== "__FALLBACK__"
        ? service.product
        : null;
    const key = product ? JSON.stringify([product.type]) : "other";
    const group = groups.get(key) ?? {
      key,
      product,
      services: [],
      versions: [],
    };
    group.services.push(service);
    const versionKey = product
      ? JSON.stringify([product.type, product.version])
      : "other";
    let version = group.versions.find((item) => item.key === versionKey);
    if (!version) {
      version = {
        key: versionKey,
        product,
        services: [],
      };
      group.versions.push(version);
    }
    version.services.push(service);
    groups.set(key, group);
  }
  return [...groups.values()];
}
