import { useState } from "react";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { InfrastructureServiceSchema } from "@multica/core/infrastructure";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider } from "../navigation";
import { ServiceCatalog } from "./service-catalog";

vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
const services = [
  {
    id: 1,
    name: "hdfs-prod",
    type: "HDFS",
    version: "3.3.6",
    health: "HEALTHY",
    product: { type: "TDH", name: "Transwarp Data Hub", version: "9.3.2" },
  },
  {
    id: 2,
    name: "search-logs",
    type: "SEARCH",
    health: "WARNING",
    product: { type: "TDH", name: "Transwarp Data Hub", version: "9.3.2" },
  },
  {
    id: 3,
    name: "inceptor-test",
    type: "INCEPTOR",
    health: "HEALTHY",
    product: { type: "TDH", name: "Transwarp Data Hub", version: "9.2.0" },
  },
  {
    id: 4,
    name: "studio-analytics",
    type: "STUDIO",
    health: "HEALTHY",
    product: { type: "STUDIO", name: "Transwarp Studio", version: "4.2.0" },
  },
  { id: 5, name: "unassigned", type: "OTHER", health: "FUTURE_STATE" },
].map((service) =>
  InfrastructureServiceSchema.parse({ installed: true, ...service }),
);

function mount(rows = services, loading = false) {
  function Content() {
    const [location, setLocation] = useState(
      "/acme/infrastructure?manager=manager-a&cluster=7",
    );
    const url = new URL(location, "https://app.test");
    return (
      <NavigationProvider
        value={{
          pathname: url.pathname,
          searchParams: url.searchParams,
          hash: "",
          push: setLocation,
          replace: setLocation,
          back: () => {},
          getShareableUrl: (path) => path,
        }}
      >
        <ServiceCatalog services={rows} loading={loading} />
        <div data-testid="location">{location}</div>
      </NavigationProvider>
    );
  }
  return renderWithI18n(<Content />);
}

describe("product catalog layout", () => {
  it("renders one product section with separate version sections and actual service versions", () => {
    mount();
    const product = screen.getByRole("region", { name: "Transwarp Data Hub" });
    expect(
      within(product).getByRole("button", { name: "Transwarp Data Hub" }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(
      within(product).getByRole("button", { name: /9\.3\.2/ }),
    ).toBeVisible();
    expect(
      within(product).getByRole("button", { name: /9\.2\.0/ }),
    ).toBeVisible();
    expect(within(product).getByText("3.3.6")).toBeVisible();
    expect(within(product).getByText("1 needs attention")).toBeVisible();
    expect(
      screen.getByRole("region", { name: "Other services" }),
    ).toHaveTextContent("Unknown");
  });
  it("collapses products and versions independently and reveals search matches", async () => {
    const user = userEvent.setup();
    mount();
    await user.click(screen.getByRole("button", { name: /9\.3\.2/ }));
    expect(
      screen.queryByRole("link", { name: "Service · hdfs-prod" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Service · inceptor-test" }),
    ).toBeVisible();
    await user.click(
      screen.getByRole("button", { name: "Transwarp Data Hub" }),
    );
    expect(
      screen.queryByRole("link", { name: "Service · inceptor-test" }),
    ).not.toBeInTheDocument();
    await user.type(
      screen.getByRole("textbox", {
        name: "Find service name, type or version...",
      }),
      "hdfs",
    );
    expect(
      screen.getByRole("link", { name: "Service · hdfs-prod" }),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Transwarp Data Hub" }),
    ).toHaveAttribute("aria-expanded", "true");
  });
  it("filters all versions of a product and keeps the Manager and cluster in service links", async () => {
    const user = userEvent.setup();
    mount();
    await user.click(screen.getByRole("combobox", { name: "Product" }));
    await user.click(
      screen.getByRole("option", { name: "Transwarp Data Hub" }),
    );
    expect(
      screen.getByRole("link", { name: "Service · hdfs-prod" }),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Service · inceptor-test" }),
    ).toBeVisible();
    expect(
      screen.queryByRole("link", { name: "Service · studio-analytics" }),
    ).not.toBeInTheDocument();
    const url = new URL(
      screen
        .getByRole("link", { name: "Service · hdfs-prod" })
        .getAttribute("href")!,
      "https://app.test",
    );
    expect(Object.fromEntries(url.searchParams)).toMatchObject({
      manager: "manager-a",
      cluster: "7",
      service: "1",
      product: '["TDH"]',
      tab: "roles",
    });
    await user.click(screen.getByRole("button", { name: "Issues only" }));
    expect(screen.getByRole("button", { name: "Issues only" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(
      screen.queryByRole("link", { name: "Service · hdfs-prod" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Service · search-logs" }),
    ).toBeVisible();
  });
  it("keeps a flat view available and shows an empty state outside the table", async () => {
    const user = userEvent.setup();
    const first = mount();
    await user.click(screen.getByRole("button", { name: "Group by product" }));
    expect(screen.getAllByRole("table")).toHaveLength(1);
    expect(
      screen.queryByRole("button", { name: "Transwarp Data Hub" }),
    ).not.toBeInTheDocument();
    first.unmount();
    mount([]);
    expect(screen.getByRole("status")).toHaveTextContent("No results");
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });
});
