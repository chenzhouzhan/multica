import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  InfrastructureOperationSchema,
  InfrastructureServiceSchema,
} from "@multica/core/infrastructure";
import { renderWithI18n } from "../test/i18n";
import { NavigationProvider } from "../navigation";
import { ServiceImages } from "./service-images";

const mocked = vi.hoisted(() => ({
  resources: vi.fn(),
  preview: vi.fn(),
  start: vi.fn(),
  query: vi.fn(),
}));
vi.mock("@multica/core/api", () => ({
  api: {
    infrastructureImageResources: mocked.resources,
    previewInfrastructureImages: mocked.preview,
    startInfrastructureImages: mocked.start,
    infrastructureQuery: mocked.query,
  },
}));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
const scope = {
  ws: "workspace-a",
  connection: "manager-a",
  cluster: 7,
  service: 8,
};
const targets = ["worker", "coordinator"].map((name) => ({
  key: `deployment/qa/${name}-a/${name}`,
  role_type: name.toUpperCase(),
  role_name: name,
  kind: "Deployment",
  namespace: "qa",
  workload: `${name}-a`,
  uid: `uid-${name}`,
  generation: 7,
  container: name,
  image: `manager.example/team/${name}:custom-tag`,
  replicas: 2,
}));
const digest = "sha256:" + "a".repeat(64);
const prepared = () =>
  InfrastructureOperationSchema.parse({
    id: "image-op",
    connection_id: scope.connection,
    cluster_id: 7,
    service_id: 8,
    kind: "publish_images",
    status: "ready",
    details: {
      items: [
        {
          target: targets[0],
          image: {
            source: "harbor.example/team/worker:Li-debug_42",
            pinned: `harbor.example/team/worker@${digest}`,
            digest,
            verification_digests: [digest],
          },
          status: "resolved",
        },
      ],
      steps: [],
    },
  });
const clients: QueryClient[] = [];

function mount(canManage = true, serviceId = 8) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  mocked.resources.mockResolvedValue({ configured: true, targets });
  mocked.preview.mockResolvedValue(prepared());
  mocked.start.mockResolvedValue({ ...prepared(), status: "succeeded" });
  mocked.query.mockResolvedValue(prepared());
  function Content() {
    const [location, setLocation] = useState(
      `/acme/infrastructure?manager=manager-a&cluster=7&service=${serviceId}&tab=images`,
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
        <ServiceImages
          key={serviceId}
          scope={{ ...scope, service: serviceId }}
          service={InfrastructureServiceSchema.parse({
            id: serviceId,
            name: `Service ${serviceId}`,
          })}
          canManage={canManage}
        />
        <span data-testid="location" hidden>
          {location}
        </span>
      </NavigationProvider>
    );
  }
  return renderWithI18n(
    <QueryClientProvider client={client}>
      <Content />
    </QueryClientProvider>,
  );
}
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear());
  vi.resetAllMocks();
});

describe("service role image publication", () => {
  it("submits only selected role images in the inherited service scope and requires confirmation", async () => {
    const user = userEvent.setup();
    mount();
    const worker = await screen.findByRole("checkbox", {
      name: "Select worker · worker-a · worker",
    });
    expect(
      screen.getByRole("textbox", {
        name: "coordinator · coordinator-a Harbor image",
      }),
    ).toBeDisabled();
    await user.click(worker);
    await user.type(
      screen.getByRole("textbox", { name: "worker · worker-a Harbor image" }),
      "harbor.example/team/worker:Li-debug_42",
    );
    await user.click(
      screen.getByRole("button", { name: /Preview selected roles/ }),
    );
    const confirm = await screen.findByRole("button", {
      name: "Confirm publication",
    });
    expect(confirm).toBeDisabled();
    expect(mocked.preview).toHaveBeenCalledWith(scope, {
      action: "publish_images",
      reason: "",
      items: [
        {
          key: targets[0]!.key,
          source: "harbor.example/team/worker:Li-debug_42",
        },
      ],
    });
    expect(mocked.start).not.toHaveBeenCalled();
    expect(
      await screen.findByText(`harbor.example/team/worker@${digest}`),
    ).toBeVisible();
    await user.click(
      screen.getByRole("checkbox", {
        name: /Confirm selected container images in Service 8/,
      }),
    );
    await user.click(confirm);
    await waitFor(() =>
      expect(mocked.start).toHaveBeenCalledExactlyOnceWith(scope, "image-op"),
    );
    expect(screen.getByTestId("location")).toHaveTextContent("service=8");
    expect(screen.getByTestId("location")).toHaveTextContent(
      "image_operation=image-op",
    );
  });
  it("keeps a failed submission visible without retrying the write", async () => {
    const user = userEvent.setup();
    mount();
    await user.click(
      await screen.findByRole("checkbox", {
        name: "Select worker · worker-a · worker",
      }),
    );
    await user.type(
      screen.getByRole("textbox", { name: "worker · worker-a Harbor image" }),
      "harbor.example/team/worker:Li-debug_42",
    );
    await user.click(
      screen.getByRole("button", { name: /Preview selected roles/ }),
    );
    await user.click(
      await screen.findByRole("checkbox", {
        name: /Confirm selected container images in Service 8/,
      }),
    );
    mocked.start.mockRejectedValueOnce(
      new Error("Deployment changed; read again"),
    );
    await user.click(
      screen.getByRole("button", { name: "Confirm publication" }),
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Deployment changed; read again",
    );
    expect(mocked.start).toHaveBeenCalledTimes(1);
    expect(
      screen.getByText(`harbor.example/team/worker@${digest}`),
    ).toBeVisible();
  });
  it("shows members current images without exposing publication or SSH administration", async () => {
    mount(false);
    expect(
      await screen.findByText("manager.example/team/worker:custom-tag"),
    ).toBeVisible();
    expect(
      screen.getByRole("checkbox", {
        name: "Select worker · worker-a · worker",
      }),
    ).toBeDisabled();
    expect(
      screen.queryByRole("button", { name: "Configure execution access" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Preview selected roles/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Only workspace owners and admins can publish images."),
    ).toBeVisible();
  });
  it("does not carry a service's selected roles or image inputs into another service", async () => {
    const user = userEvent.setup();
    const first = mount();
    await user.click(
      await screen.findByRole("checkbox", {
        name: "Select worker · worker-a · worker",
      }),
    );
    await user.type(
      screen.getByRole("textbox", { name: "worker · worker-a Harbor image" }),
      "harbor.example/team/worker:private-draft",
    );
    first.unmount();
    mount(true, 9);
    expect(
      await screen.findByRole("textbox", {
        name: "worker · worker-a Harbor image",
      }),
    ).toHaveValue("");
    expect(
      screen.getByRole("checkbox", {
        name: "Select worker · worker-a · worker",
      }),
    ).not.toBeChecked();
    expect(mocked.resources).toHaveBeenCalledWith({ ...scope, service: 9 });
  });
});
