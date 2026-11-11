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
      service.product && service.product.type !== "__FALLBACK__"
        ? JSON.stringify([service.product.type, service.product.version])
        : "other";
    return (
      (!search ||
        [
          service.name,
          service.type,
          service.type_name,
          service.version,
          service.product?.name,
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
}

export function groupInfrastructureProducts(
  services: InfrastructureService[],
): InfrastructureProductGroup[] {
  const groups = new Map<string, InfrastructureProductGroup>();
  for (const service of services) {
    const product = service.product;
    const key =
      product && product.type !== "__FALLBACK__"
        ? JSON.stringify([product.type, product.version])
        : "other";
    const group = groups.get(key) ?? {
      key,
      product: key === "other" ? null : product,
      services: [],
    };
    group.services.push(service);
    groups.set(key, group);
  }
  return [...groups.values()];
}
