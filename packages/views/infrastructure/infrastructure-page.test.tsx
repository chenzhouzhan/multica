import { useState } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../test/i18n";
import { InfrastructurePage } from "./infrastructure-page";
import { ClustersPanel } from "./clusters-panel";
import {
  InfrastructureServiceSchema,
  useMetainfoDraftStore,
  useInfrastructureEditorStore,
} from "@multica/core/infrastructure";
import { NavigationProvider } from "../navigation";

const mocked = vi.hoisted(() => ({
  list: vi.fn(),
  query: vi.fn(),
  grants: vi.fn(),
  save: vi.fn(),
  remove: vi.fn(),
  metaFiles: vi.fn(),
  metaFile: vi.fn(),
  metaSave: vi.fn(),
  images: vi.fn(),
}));
vi.mock("@multica/ui/components/ui/code-editor", () => ({
  CodeEditor: ({
    value,
    onChange,
    readOnly,
    language,
    "aria-label": label,
  }: import("@multica/ui/components/ui/code-editor").CodeEditorProps) => (
    <div>
      <textarea
        aria-label={label}
        aria-readonly={readOnly}
        readOnly={readOnly}
        value={value}
        data-code-language={language}
        onChange={(event) => onChange(event.target.value)}
      />
      <div className="cm-search">
        <input aria-label="Find in file" />
      </div>
    </div>
  ),
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-a" }));
vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/paths")>();
  return { ...actual, useWorkspacePaths: () => actual.paths.workspace("acme") };
});
vi.mock("@multica/core/api", () => ({
  api: {
    serviceMetaFiles: mocked.metaFiles,
    serviceMetaFile: mocked.metaFile,
    saveServiceMetaFile: mocked.metaSave,
    listInfrastructureConnections: mocked.list,
    infrastructureQuery: mocked.query,
    setInfrastructureGrants: mocked.grants,
    saveInfrastructureConnection: mocked.save,
    deleteInfrastructureConnection: mocked.remove,
    infrastructureImageResources: mocked.images,
  },
}));
const connections = ["a", "b"].map((id) => ({
  id,
  name: `Manager ${id.toUpperCase()}`,
  url: `https://manager-${id}.test`,
  username: "ops",
  ca_pem: "",
  restart_grants: [],
}));
const clients: QueryClient[] = [];
function TestNavigation({
  initial = "/acme/infrastructure",
}: {
  initial?: string;
}) {
  const [location, setLocation] = useState(initial);
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
      <aside aria-label="Cluster list">
        <ClustersPanel />
      </aside>
      <InfrastructurePage />
      <div data-testid="location" hidden>
        {location}
      </div>
    </NavigationProvider>
  );
}
function mount(canManage = false, initial?: string) {
  mocked.metaFiles.mockResolvedValue([
    {
      key: "TEST/v1/i18n/service_en_US.properties",
      name: "i18n/service_en_US.properties",
      type: "I18N_FILE",
    },
    { key: "TEST/v1/metainfo.yaml", name: "metainfo.yaml", type: "INFO_YAML" },
  ]);
  mocked.metaFile.mockImplementation(
    async (
      _ws: string,
      _connection: string,
      _product: unknown,
      key: string,
    ) => ({
      key,
      text: true,
      content: key.endsWith(".yaml") ? "name: service\n" : "name=Service\n",
    }),
  );
  mocked.metaSave.mockResolvedValue({ saved: true });
  mocked.list.mockResolvedValue({
    connections,
    configured: true,
    can_manage: canManage,
  });
  mocked.query.mockImplementation(
    async (_ws: string, tool: string, input: { connection_id: string }) => {
      const node = {
        id: 1,
        name: `host-${input.connection_id}`,
        ip: "10.0.0.1",
        cluster_id: 7,
        cluster_name: "Cluster",
        status: "ONLINE",
        cores: 8,
        memory: 1024,
        disk: 2048,
      };
      switch (tool) {
        case "list_clusters":
          return [{ id: 7, name: "Cluster" }];
        case "list_nodes":
          return [node];
        case "list_services":
          return [
            InfrastructureServiceSchema.parse({
              product: {
                type: "STUDIO",
                name: "Studio",
                version: "v1",
                display_version: "Version 1",
              },
              id: 8,
              name: "Service",
              type: "TEST",
              cluster_id: 7,
              health: "HEALTHY",
              installed: true,
              busy: false,
            }),
          ];
        case "service_relations":
          return {
            dependencies: [
              InfrastructureServiceSchema.parse({
                id: 1,
                name: "TOS",
                cluster_id: 0,
              }),
            ],
            dependents: [
              InfrastructureServiceSchema.parse({
                id: 10,
                name: "Consumer",
                cluster_id: 7,
              }),
            ],
          };
        case "list_roles":
          return [
            {
              id: 9,
              name: "Worker",
              type: "WORKER",
              service_id: 8,
              service_name: "Service",
              node,
              health: "HEALTHY",
              status: "RUNNING",
              installed: true,
            },
          ];
        default:
          return [];
      }
    },
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  return renderWithI18n(
    <QueryClientProvider client={client}>
      <TestNavigation initial={initial} />
    </QueryClientProvider>,
  );
}
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear());
  useMetainfoDraftStore.getState().clearDraft();
  useInfrastructureEditorStore.getState().setGuard(null);
  vi.resetAllMocks();
});

async function openService(user: ReturnType<typeof userEvent.setup>) {
  await user.click(
    await screen.findByRole("link", { name: "Service · Service" }),
  );
  await screen.findByRole("heading", { name: "Service roles" });
}
async function switchManager(
  user: ReturnType<typeof userEvent.setup>,
  name: string,
) {
  await user.click(screen.getByRole("combobox", { name: "Switch Manager" }));
  await user.click(await screen.findByRole("option", { name }));
}
async function openMeta(user: ReturnType<typeof userEvent.setup>) {
  await openService(user);
  await user.click(screen.getByRole("link", { name: "Metainfo" }));
  return screen.findByRole("textbox", { name: "File content" });
}

describe("infrastructure page", () => {
  it("opens image publication from the current service role and selects its actual container", async () => {
    const user = userEvent.setup();
    mocked.images.mockResolvedValue({
      configured: true,
      targets: [
        {
          key: "deployment/qa/worker-a/worker",
          role_type: "WORKER",
          role_name: "Worker",
          kind: "Deployment",
          namespace: "qa",
          workload: "worker-a",
          uid: "uid-worker",
          generation: 7,
          container: "worker",
          image: "registry.example/team/worker:custom",
          replicas: 2,
          blocked_reason: "",
        },
      ],
    });
    mount(true);
    await openService(user);
    await user.click(screen.getByRole("link", { name: "Update image" }));
    expect(
      await screen.findByRole("checkbox", {
        name: "Select Worker · worker-a · worker",
      }),
    ).toBeChecked();
    expect(mocked.images).toHaveBeenCalledWith({
      ws: "workspace-a",
      connection: "a",
      cluster: 7,
      service: 8,
    });
    expect(screen.getByTestId("location")).toHaveTextContent("service=8");
    expect(screen.getByTestId("location")).toHaveTextContent("tab=images");
    expect(screen.getByTestId("location")).toHaveTextContent(
      "role_type=WORKER",
    );
  });
  it("uses a full service list and preserves filters when returning from details", async () => {
    const user = userEvent.setup();
    mount();
    const search = await screen.findByRole("textbox", {
      name: "Find service name, type or version…",
    });
    expect(
      mocked.query.mock.calls.some(([, tool]) => tool === "list_roles"),
    ).toBe(false);
    await user.type(search, "Service");
    await openService(user);
    expect(
      screen.queryByRole("textbox", {
        name: "Find service name, type or version…",
      }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("link", { name: "Back to services" }));
    expect(
      await screen.findByRole("textbox", {
        name: "Find service name, type or version…",
      }),
    ).toHaveValue("Service");
    expect(screen.getByTestId("location")).toHaveTextContent("q=Service");
  });
  it("separates identical service and role ids when switching Managers", async () => {
    const user = userEvent.setup();
    mount();
    await openService(user);
    await screen.findByText("host-a", { selector: "td" });
    expect(
      screen.queryByRole("button", { name: "Add Manager" }),
    ).not.toBeInTheDocument();
    await switchManager(user, "Manager B");
    await openService(user);
    await screen.findByText("host-b", { selector: "td" });
    expect(
      screen.queryByText("host-a", { selector: "td" }),
    ).not.toBeInTheDocument();
    expect(mocked.query).toHaveBeenCalledWith(
      "workspace-a",
      "list_roles",
      expect.objectContaining({
        connection_id: "b",
        cluster_id: 7,
        service_id: 8,
      }),
      expect.anything(),
    );
  });
  it("keeps restart permission scoped to the selected Manager, cluster, service and role", async () => {
    const user = userEvent.setup();
    mount(true);
    await openService(user);
    expect(screen.getByRole("button", { name: "Restart role" })).toBeDisabled();
    await user.click(
      await screen.findByRole("button", { name: "Not authorized" }),
    );
    mocked.grants.mockResolvedValue(connections[0]);
    await user.click(
      screen.getByRole("checkbox", {
        name: "Allow automatic restart · Worker",
      }),
    );
    await waitFor(() =>
      expect(mocked.grants).toHaveBeenCalledWith("workspace-a", "a", [
        { cluster_id: 7, service_id: 8, role_id: 9 },
      ]),
    );
  });
  it("moves connection administration into Manager settings and retains TLS controls", async () => {
    const user = userEvent.setup();
    mount(true);
    await user.click(
      await screen.findByRole("button", { name: "Manage Managers" }),
    );
    const dialog = await screen.findByRole("dialog", {
      name: "Manage Managers",
    });
    await user.click(
      within(dialog).getAllByRole("button", { name: "Edit Manager" })[0]!,
    );
    const toggle = screen.getByRole("checkbox", {
      name: "Skip certificate verification",
    });
    expect(toggle).not.toBeChecked();
    await user.click(toggle);
    expect(
      screen.getByLabelText("CA certificate (PEM, optional)"),
    ).toBeDisabled();
    mocked.save.mockResolvedValue({ ...connections[0], skip_tls_verify: true });
    await user.click(screen.getByRole("button", { name: "Connect and save" }));
    await waitFor(() =>
      expect(mocked.save).toHaveBeenCalledWith(
        "workspace-a",
        "a",
        expect.objectContaining({ skip_tls_verify: true }),
      ),
    );
  });
  it("fetches dependencies only from the service detail action", async () => {
    const user = userEvent.setup();
    mount();
    expect(
      await screen.findByRole("button", { name: "Studio" }),
    ).toBeInTheDocument();
    await openService(user);
    expect(
      mocked.query.mock.calls.some(([, tool]) => tool === "service_relations"),
    ).toBe(false);
    await user.click(
      screen.getByRole("button", { name: "Service dependencies" }),
    );
    expect(
      await screen.findByRole("dialog", {
        name: "Service · Service dependencies",
      }),
    ).toBeInTheDocument();
    expect(await screen.findByText("TOS", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("Consumer", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("Depends on", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("Used by", { exact: true })).toBeInTheDocument();
    await user.keyboard("{Escape}");
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(
      screen.getByRole("heading", { name: "Service roles" }),
    ).toBeInTheDocument();
  });
  it("loads inline metainfo on demand and keeps members read-only", async () => {
    const user = userEvent.setup();
    mount();
    const editor = await openMeta(user);
    expect(editor).toHaveValue("name: service\n");
    expect(editor).toHaveAttribute("aria-readonly", "true");
    expect(
      screen.queryByRole("button", { name: "Edit file" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Inspect cluster" }),
    ).not.toBeInTheDocument();
    expect(mocked.metaFile).toHaveBeenCalledWith(
      "workspace-a",
      "a",
      8,
      "TEST/v1/metainfo.yaml",
    );
    await user.click(screen.getByRole("button", { name: "i18n" }));
    await user.click(
      screen.getByRole("button", { name: "i18n/service_en_US.properties" }),
    );
    await waitFor(() =>
      expect(screen.getByRole("textbox", { name: "File content" })).toHaveValue(
        "name=Service\n",
      ),
    );
    await switchManager(user, "Manager B");
    await openMeta(user);
    expect(mocked.metaFile).toHaveBeenCalledWith(
      "workspace-a",
      "b",
      8,
      "TEST/v1/metainfo.yaml",
    );
  });
  it("preserves failed drafts, protects navigation, and saves the selected service version", async () => {
    const user = userEvent.setup();
    mount(true);
    const editor = await openMeta(user);
    await user.click(screen.getByRole("button", { name: "Edit file" }));
    await user.clear(editor);
    await user.type(editor, "name: updated");
    mocked.metaSave.mockRejectedValueOnce(
      new Error("Manager returned HTTP 500"),
    );
    await user.click(screen.getByRole("button", { name: "Save to Manager" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Manager returned HTTP 500",
    );
    expect(editor).toHaveValue("name: updated");
    await user.click(screen.getByRole("link", { name: "Back to services" }));
    expect(
      await screen.findByRole("dialog", { name: "Discard unsaved changes?" }),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Keep editing" }));
    await user.click(screen.getByRole("button", { name: "Save to Manager" }));
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Saved to Manager",
    );
    expect(mocked.metaSave).toHaveBeenLastCalledWith(
      "workspace-a",
      "a",
      8,
      "TEST/v1/metainfo.yaml",
      "name: updated",
    );
    expect(editor).toHaveValue("name: updated");
    expect(editor).toHaveAttribute("aria-readonly", "true");
    await user.click(screen.getByRole("link", { name: "Back to services" }));
    expect(
      await screen.findByRole("textbox", {
        name: "Find service name, type or version…",
      }),
    ).toBeVisible();
  });
  it("confirms changing files with edits and restores a draft after remount", async () => {
    const user = userEvent.setup();
    const first = mount(true);
    const editor = await openMeta(user);
    await user.click(screen.getByRole("button", { name: "Edit file" }));
    await user.clear(editor);
    await user.type(editor, "name: recovery");
    await user.click(screen.getByRole("button", { name: "i18n" }));
    await user.click(
      screen.getByRole("button", { name: "i18n/service_en_US.properties" }),
    );
    expect(
      await screen.findByRole("dialog", { name: "Discard unsaved changes?" }),
    ).toBeVisible();
    await user.click(screen.getByRole("button", { name: "Keep editing" }));
    first.unmount();
    mount(
      true,
      "/acme/infrastructure?manager=a&cluster=7&service=8&tab=metainfo",
    );
    expect(
      await screen.findByRole("textbox", { name: "File content" }),
    ).toHaveValue("name: recovery");
    await user.click(screen.getByRole("link", { name: "Back to services" }));
    await user.click(
      await screen.findByRole("button", { name: "Discard changes" }),
    );
    expect(
      await screen.findByRole("textbox", {
        name: "Find service name, type or version…",
      }),
    ).toBeVisible();
  });
  it("opens role logs without loading operation history on the default page", async () => {
    const user = userEvent.setup();
    mount();
    await openService(user);
    expect(
      mocked.query.mock.calls.some(([, tool]) => tool === "list_operations"),
    ).toBe(false);
    await user.click(await screen.findByRole("link", { name: "View logs" }));
    await waitFor(() =>
      expect(mocked.query).toHaveBeenCalledWith(
        "workspace-a",
        "search_logs",
        expect.objectContaining({ connection_id: "a", hostname: "host-a" }),
        expect.anything(),
      ),
    );
  });
});
