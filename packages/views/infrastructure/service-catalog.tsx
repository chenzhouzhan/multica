"use client";

import { useEffect, useState } from "react";
import {
  filterInfrastructureServices,
  groupInfrastructureProducts,
  infrastructureHref,
  type InfrastructureService,
} from "@multica/core/infrastructure";
import { useWorkspacePaths } from "@multica/core/paths";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@multica/ui/components/ui/select";
import {
  ArrowRight,
  ChevronDown,
  Layers,
  Search,
  Server,
} from "@multica/ui/icons";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AppLink, useIsNavigating } from "../navigation";
import { useInfrastructureNavigation } from "./use-infrastructure-navigation";

export function ServiceCatalog({
  services,
  loading,
}: {
  services: InfrastructureService[];
  loading: boolean;
}) {
  const { t } = useT("runtimes");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const nav = useInfrastructureNavigation();
  const p = useWorkspacePaths();
  const urlSearch = nav.searchParams.get("q") ?? "";
  const [search, setSearch] = useState(urlSearch);
  const navigating = useIsNavigating();
  useEffect(() => {
    if (!navigating) setSearch(urlSearch);
  }, [urlSearch, navigating]);
  const health = nav.searchParams.get("health") ?? "all";
  const product = nav.searchParams.get("product") ?? "";
  const grouped = nav.searchParams.get("group") !== "none";
  const products = groupInfrastructureProducts(services);
  const filtered = filterInfrastructureServices(services, {
    search,
    health,
    product,
  });
  const groups = grouped
    ? groupInfrastructureProducts(filtered)
    : [{ key: "all", product: null, services: filtered }];
  const filtering = !!search || health !== "all" || !!product;
  const set = (patch: Record<string, string | number | null>) =>
    nav.replace(
      infrastructureHref(p.infrastructure(), nav.searchParams, {
        q: search,
        ...patch,
      }),
    );
  const serviceHref = (id: number) =>
    infrastructureHref(p.infrastructure(), nav.searchParams, {
      service: id,
      tab: "roles",
      view: null,
      q: search,
    });
  const productLabel = (group: (typeof products)[number]) =>
    group.product
      ? [
          group.product.name || group.product.type,
          group.product.display_version || group.product.version,
        ]
          .filter(Boolean)
          .join(" · ")
      : t(($) => $.infrastructure.other_product);
  const healthItems = [
    { value: "all", label: t(($) => $.infrastructure.all_health) },
    { value: "unhealthy", label: t(($) => $.infrastructure.non_healthy) },
  ];
  const productItems = [
    { value: "all", label: t(($) => $.infrastructure.all_products) },
    ...products.map((group) => ({
      value: group.key,
      label: productLabel(group),
    })),
  ];
  const rows = (items: InfrastructureService[]) =>
    items.map((service) => (
      <tr
        key={service.id}
        className="border-b border-border/70 last:border-b-0 hover:bg-surface-hover"
      >
        <td className="min-w-40 px-3 py-4">
          <AppLink
            href={serviceHref(service.id)}
            className="flex items-start gap-3 rounded-sm outline-none hover:text-brand focus-visible:outline-2 focus-visible:outline-ring"
          >
            <Server className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
            <span className="min-w-0">
              <span className="block break-words font-medium">
                {service.name}
              </span>
              <span className="mt-0.5 block break-words text-caption text-muted-foreground">
                {service.type_name || service.type}
              </span>
            </span>
          </AppLink>
        </td>
        <td
          className={cn(
            "whitespace-nowrap px-3 py-4 text-caption",
            service.health === "HEALTHY"
              ? "text-success"
              : service.health === "DOWN"
                ? "text-destructive"
                : service.health === "WARNING"
                  ? "text-warning"
                  : "text-muted-foreground",
          )}
        >
          {service.health}
        </td>
        <td className="whitespace-nowrap px-3 py-4 text-caption text-muted-foreground">
          {t(($) =>
            service.installed
              ? $.infrastructure.installed
              : $.infrastructure.not_installed,
          )}
        </td>
        <td className="break-words px-3 py-4 font-mono text-caption text-muted-foreground">
          {service.version}
        </td>
        <td className="px-3 py-4">
          <AppLink
            href={serviceHref(service.id)}
            aria-label={`${t(($) => $.infrastructure.service)} · ${service.name}`}
            className="inline-flex size-7 items-center justify-center rounded-md text-muted-foreground outline-none hover:bg-surface-selected hover:text-surface-selected-foreground focus-visible:outline-2 focus-visible:outline-ring"
          >
            <ArrowRight className="size-4" />
          </AppLink>
        </td>
      </tr>
    ));

  return (
    <section
      className="grid min-w-0 gap-4"
      aria-label={t(($) => $.infrastructure.products_services)}
    >
      <div className="flex flex-wrap items-center gap-2.5">
        <div className="relative min-w-48 max-w-sm flex-1">
          <Search className="pointer-events-none absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
          <Input
            aria-label={t(($) => $.infrastructure.search_services)}
            placeholder={t(($) => $.infrastructure.search_services)}
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              set({ q: event.target.value });
            }}
            className="pl-8"
          />
        </div>
        <Select
          value={health}
          items={healthItems}
          onValueChange={(value) => {
            if (value) set({ health: value === "all" ? null : value });
          }}
        >
          <SelectTrigger
            aria-label={t(($) => $.infrastructure.health)}
            className="max-w-48"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {healthItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={product || "all"}
          items={productItems}
          onValueChange={(value) => {
            if (value) set({ product: value === "all" ? null : value });
          }}
        >
          <SelectTrigger
            aria-label={t(($) => $.infrastructure.product)}
            className="max-w-64"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {productItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button
          variant={grouped ? "secondary" : "ghost"}
          aria-pressed={grouped}
          onClick={() => set({ group: grouped ? "none" : null })}
        >
          <Layers />
          {t(($) => $.infrastructure.group_products)}
        </Button>
      </div>
      {filtering && (
        <div className="flex items-center gap-3 text-caption text-muted-foreground">
          <span>
            {t(($) => $.infrastructure.results_count, {
              count: filtered.length,
            })}
          </span>
          <Button
            variant="ghost"
            size="xs"
            onClick={() => set({ q: null, health: null, product: null })}
          >
            {t(($) => $.infrastructure.clear_filters)}
          </Button>
        </div>
      )}
      <div className="min-w-0 overflow-x-auto">
        <table className="w-full text-left text-body">
          <colgroup>
            <col className="w-[36%]" />
            <col className="w-[17%]" />
            <col className="w-[17%]" />
            <col />
            <col className="w-12" />
          </colgroup>
          <thead className="border-b bg-muted/40 text-caption text-muted-foreground">
            <tr>
              {[
                t(($) => $.infrastructure.service),
                t(($) => $.infrastructure.health),
                t(($) => $.infrastructure.deployment),
                t(($) => $.infrastructure.version),
                "",
              ].map((label, index) => (
                <th
                  key={index}
                  className="whitespace-nowrap px-3 py-3 font-medium"
                >
                  {label}
                </th>
              ))}
            </tr>
          </thead>
          {groups.map((group) => (
            <tbody key={group.key}>
              {grouped && (
                <tr>
                  <td colSpan={5} className="pb-1 pt-4">
                    <Button
                      variant="ghost"
                      aria-expanded={!collapsed[group.key] || !!search}
                      className="h-auto w-full justify-start gap-2 whitespace-normal px-3 py-2 text-left text-caption aria-expanded:bg-transparent aria-expanded:text-foreground dark:aria-expanded:bg-transparent hover:bg-surface-hover dark:hover:bg-surface-hover"
                      onClick={() =>
                        setCollapsed((previous) => ({
                          ...previous,
                          [group.key]: !previous[group.key],
                        }))
                      }
                    >
                      <ChevronDown
                        className={cn(
                          "size-3",
                          collapsed[group.key] && !search && "-rotate-90",
                        )}
                      />
                      <span className="flex size-5 shrink-0 items-center justify-center rounded-md bg-brand/10 text-brand">
                        <Layers className="size-3.5" />
                      </span>
                      <span className="min-w-0 break-words">
                        {productLabel(group)}
                      </span>
                      <span className="text-caption font-normal text-muted-foreground">
                        {group.services.length}
                      </span>
                    </Button>
                  </td>
                </tr>
              )}
              {(!grouped || !collapsed[group.key] || !!search) &&
                rows(group.services)}
            </tbody>
          ))}
          {(loading || !filtered.length) && (
            <tbody>
              <tr>
                <td
                  colSpan={5}
                  className="px-3 py-12 text-center text-body text-muted-foreground"
                >
                  {t(($) =>
                    loading
                      ? $.infrastructure.loading
                      : $.infrastructure.no_results,
                  )}
                </td>
              </tr>
            </tbody>
          )}
        </table>
      </div>
    </section>
  );
}
