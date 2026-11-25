"use client";

import { Fragment, useEffect, useId, useState } from "react";
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
import { ArrowRight, ChevronDown, Layers, Search } from "@multica/ui/icons";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../i18n";
import { AppLink, useIsNavigating } from "../navigation";
import { useInfrastructureNavigation } from "./use-infrastructure-navigation";

function ServiceHealth({ health }: { health: string }) {
  const { t } = useT("runtimes");
  const status =
    health === "HEALTHY"
      ? "healthy"
      : health === "WARNING"
        ? "warning"
        : health === "DOWN"
          ? "down"
          : "unknown";
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 whitespace-nowrap text-caption",
        status === "warning" && "text-warning",
        status === "down" && "text-destructive",
        status === "unknown" && "text-muted-foreground",
      )}
    >
      <span
        aria-hidden="true"
        className={cn(
          "size-1.5 shrink-0 rounded-full",
          status === "healthy"
            ? "bg-success"
            : status === "warning"
              ? "bg-warning"
              : status === "down"
                ? "bg-destructive"
                : "bg-muted-foreground",
        )}
      />
      {t(($) => $.infrastructure.catalog.health[status])}
    </span>
  );
}

export function ServiceCatalog({
  services,
  loading,
}: {
  services: InfrastructureService[];
  loading: boolean;
}) {
  const { t } = useT("runtimes");
  const id = useId();
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
  const groups = groupInfrastructureProducts(filtered);
  const filtering = !!search || health !== "all" || !!product;
  const set = (patch: Record<string, string | number | null>) =>
    nav.replace(
      infrastructureHref(p.infrastructure(), nav.searchParams, {
        q: search,
        ...patch,
      }),
    );
  const toggle = (key: string) =>
    setCollapsed((previous) => ({ ...previous, [key]: !previous[key] }));
  const serviceHref = (service: number) =>
    infrastructureHref(p.infrastructure(), nav.searchParams, {
      service,
      tab: "roles",
      view: null,
      role_type: null,
      image_operation: null,
      q: search,
    });
  const productName = (group: (typeof products)[number]) =>
    group.product
      ? group.product.name || group.product.type
      : t(($) => $.infrastructure.other_product);
  const productItems = [
    { value: "all", label: t(($) => $.infrastructure.all_products) },
    ...products.map((group) => ({
      value: group.key,
      label: productName(group),
    })),
  ];
  const columns = (
    <colgroup>
      <col className="w-[60%] sm:w-[48%]" />
      <col className="w-[28%] sm:w-[24%]" />
      <col className="w-0 sm:w-[20%]" />
      <col className="w-[12%] sm:w-[8%]" />
    </colgroup>
  );
  const headings = (
    <thead className="border-y border-surface-border bg-surface-hover/50 text-caption text-muted-foreground">
      <tr>
        <th className="px-4 py-2 text-left font-normal">
          {t(($) => $.infrastructure.service)}
        </th>
        <th className="whitespace-nowrap px-2 py-2 text-left font-normal sm:px-4">
          {t(($) => $.infrastructure.health)}
        </th>
        <th className="hidden whitespace-nowrap px-4 py-2 text-left font-normal sm:table-cell">
          {t(($) => $.infrastructure.deployment)}
        </th>
        <th>
          <span className="sr-only">{t(($) => $.infrastructure.service)}</span>
        </th>
      </tr>
    </thead>
  );
  const rows = (items: InfrastructureService[]) =>
    items.map((service) => (
      <tr
        key={service.id}
        className="border-b border-surface-border last:border-b-0 hover:bg-surface-hover/70"
      >
        <td className="px-4 py-3">
          <AppLink
            href={serviceHref(service.id)}
            className="block min-w-0 rounded-sm outline-none hover:text-brand focus-visible:outline-2 focus-visible:outline-solid focus-visible:outline-ring"
          >
            <span className="block break-words text-body font-medium">
              {service.name}
            </span>
            <span className="mt-0.5 flex flex-wrap items-center gap-x-1.5 text-caption text-muted-foreground">
              <span>{service.type_name || service.type}</span>
              {service.version && (
                <>
                  <span aria-hidden="true">·</span>
                  <span className="break-all font-mono">{service.version}</span>
                </>
              )}
            </span>
          </AppLink>
        </td>
        <td className="px-2 py-3 sm:px-4">
          <ServiceHealth health={service.health} />
        </td>
        <td className="hidden px-4 py-3 text-caption text-muted-foreground sm:table-cell">
          {t(($) =>
            service.installed
              ? $.infrastructure.installed
              : $.infrastructure.not_installed,
          )}
        </td>
        <td className="py-3 pr-2 text-center">
          <AppLink
            href={serviceHref(service.id)}
            aria-label={`${t(($) => $.infrastructure.service)} · ${service.name}`}
            className="inline-flex size-7 items-center justify-center rounded-md text-muted-foreground outline-none hover:bg-surface-selected hover:text-surface-selected-foreground focus-visible:outline-2 focus-visible:outline-solid focus-visible:outline-ring"
          >
            <ArrowRight className="size-3.5" />
          </AppLink>
        </td>
      </tr>
    ));
  return (
    <section
      className="grid min-w-0 gap-5"
      aria-label={t(($) => $.infrastructure.products_services)}
    >
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-40 flex-1 basis-full sm:basis-auto">
          <Search className="pointer-events-none absolute left-2.5 top-2 size-3.5 text-muted-foreground" />
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
        <Button
          variant={health === "unhealthy" ? "brandSubtle" : "outline"}
          size="sm"
          aria-pressed={health === "unhealthy"}
          onClick={() =>
            set({ health: health === "unhealthy" ? null : "unhealthy" })
          }
        >
          <span
            className="size-1.5 rounded-full bg-warning"
            aria-hidden="true"
          />
          {t(($) => $.infrastructure.non_healthy)}
        </Button>
        <Select
          value={product || "all"}
          items={productItems}
          onValueChange={(value) => {
            if (value) set({ product: value === "all" ? null : value });
          }}
        >
          <SelectTrigger
            aria-label={t(($) => $.infrastructure.product)}
            className="w-auto min-w-32 max-w-56"
          >
            <SelectValue className="truncate" />
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
          variant={grouped ? "brandSubtle" : "ghost"}
          size="icon-sm"
          aria-label={t(($) => $.infrastructure.group_products)}
          aria-pressed={grouped}
          onClick={() => set({ group: grouped ? "none" : null })}
        >
          <Layers className="size-3.5" />
        </Button>
      </div>
      {loading || !filtered.length ? (
        <p
          role="status"
          className="py-14 text-center text-body text-muted-foreground"
        >
          {t(($) =>
            loading ? $.infrastructure.loading : $.infrastructure.no_results,
          )}
        </p>
      ) : grouped ? (
        <div className="grid gap-5">
          {groups.map((group, groupIndex) => {
            const name = productName(group);
            const expanded = !collapsed[group.key] || !!search;
            const tableId = `${id}-product-${groupIndex}`;
            const attention = group.services.filter(
              (service) => service.health !== "HEALTHY",
            ).length;
            return (
              <section
                key={group.key}
                aria-label={name}
                className="min-w-0 overflow-hidden rounded-xl border border-surface-border bg-surface"
              >
                <header className="flex items-center justify-between gap-3 p-4">
                  <div className="min-w-0 flex-1">
                    <h2>
                      <Button
                        variant="ghost"
                        aria-label={name}
                        aria-expanded={expanded}
                        aria-controls={tableId}
                        onClick={() => toggle(group.key)}
                        className="h-auto w-full min-w-0 justify-start gap-3 whitespace-normal p-0 text-left aria-expanded:bg-transparent dark:aria-expanded:bg-transparent hover:bg-transparent dark:hover:bg-transparent"
                      >
                        <span
                          aria-hidden="true"
                          className="flex size-9 shrink-0 items-center justify-center rounded-lg border border-surface-border bg-surface-hover/60 font-mono text-caption text-muted-foreground"
                        >
                          {group.product?.type.slice(0, 3).toUpperCase() || "—"}
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="block break-words text-body font-medium">
                            {name}
                          </span>
                          {group.product && (
                            <span className="mt-0.5 block text-caption font-normal text-muted-foreground">
                              {t(
                                ($) => $.infrastructure.catalog.versions_count,
                                { count: group.versions.length },
                              )}
                            </span>
                          )}
                        </span>
                        <ChevronDown
                          aria-hidden="true"
                          className={cn(
                            "size-3.5 shrink-0 text-muted-foreground transition-transform",
                            !expanded && "-rotate-90",
                          )}
                        />
                      </Button>
                    </h2>
                  </div>
                  <div className="flex shrink-0 flex-wrap items-center justify-end gap-x-3 gap-y-1 text-caption text-muted-foreground">
                    <span className="hidden sm:inline">
                      {t(($) => $.infrastructure.services_count, {
                        count: group.services.length,
                      })}
                    </span>
                    <span
                      className={cn(
                        "inline-flex items-center gap-1.5",
                        attention > 0 && "text-warning",
                      )}
                    >
                      {attention > 0 && (
                        <span
                          className="size-1.5 rounded-full bg-warning"
                          aria-hidden="true"
                        />
                      )}
                      {attention
                        ? t(($) => $.infrastructure.catalog.attention_count, {
                            count: attention,
                          })
                        : t(($) => $.infrastructure.catalog.all_healthy)}
                    </span>
                  </div>
                </header>
                <div id={tableId} hidden={!expanded}>
                  <table className="w-full table-fixed text-left">
                    {columns}
                    {headings}
                    {group.versions.map((version, versionIndex) => {
                      const expandedVersion =
                        !collapsed[version.key] || !!search;
                      const versionId = `${tableId}-version-${versionIndex}`;
                      const versionName =
                        version.product?.display_version ||
                        version.product?.version ||
                        t(($) => $.infrastructure.catalog.version_unknown);
                      return (
                        <Fragment key={version.key}>
                          {group.product && (
                            <tbody>
                              <tr>
                                <th
                                  colSpan={4}
                                  className="px-3 pb-0 pt-2 text-left"
                                >
                                  <Button
                                    variant="ghost"
                                    size="xs"
                                    aria-expanded={expandedVersion}
                                    aria-controls={versionId}
                                    onClick={() => toggle(version.key)}
                                    className="max-w-full gap-2 text-caption aria-expanded:bg-transparent dark:aria-expanded:bg-transparent"
                                  >
                                    <ChevronDown
                                      className={cn(
                                        "size-3 shrink-0 text-muted-foreground",
                                        !expandedVersion && "-rotate-90",
                                      )}
                                    />
                                    <span className="min-w-0 truncate font-mono font-medium">
                                      {versionName}
                                    </span>
                                    <span className="font-normal text-muted-foreground">
                                      ·{" "}
                                      {t(
                                        ($) => $.infrastructure.services_count,
                                        { count: version.services.length },
                                      )}
                                    </span>
                                  </Button>
                                </th>
                              </tr>
                            </tbody>
                          )}
                          <tbody
                            id={versionId}
                            hidden={group.product ? !expandedVersion : false}
                          >
                            {rows(version.services)}
                          </tbody>
                        </Fragment>
                      );
                    })}
                  </table>
                </div>
              </section>
            );
          })}
        </div>
      ) : (
        <div className="min-w-0 overflow-hidden rounded-xl border border-surface-border bg-surface">
          <table className="w-full table-fixed text-left">
            {columns}
            {headings}
            <tbody>{rows(filtered)}</tbody>
          </table>
        </div>
      )}
      {filtering && (
        <div className="flex flex-wrap items-center justify-between gap-2 text-caption text-muted-foreground">
          <span>
            {t(($) => $.infrastructure.catalog.showing, {
              shown: filtered.length,
              total: services.length,
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
    </section>
  );
}
