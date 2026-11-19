import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { renderWithI18n } from "../test/i18n";
import { ExecutionAccessDialog } from "./execution-access-dialog";

const mocked = vi.hoisted(() => ({ get: vi.fn(), save: vi.fn() }));
vi.mock("@multica/core/api", () => ({
  api: {
    infrastructureExecutionAccess: mocked.get,
    saveInfrastructureExecutionAccess: mocked.save,
  },
}));
const info = {
  configured: true,
  host: "manager.example",
  port: 22,
  username: "root",
  host_key: "ssh-ed25519 public",
  host_key_fingerprint: "SHA256:" + "a".repeat(43),
  context: "previous",
  auth_type: "password",
  requires_context_selection: false,
  context_options: [],
};
const clients: QueryClient[] = [];
function mount(value = info) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  clients.push(client);
  mocked.get.mockResolvedValue(value);
  return renderWithI18n(
    <QueryClientProvider client={client}>
      <ExecutionAccessDialog ws="ws" connection="manager" cluster={7} />
    </QueryClientProvider>,
  );
}
afterEach(() => {
  clients.splice(0).forEach((client) => client.clear());
  vi.resetAllMocks();
});
describe("execution connection discovery", () => {
  it("reads a new server identity without a public-key field and submits the key only after trust", async () => {
    const user = userEvent.setup();
    const observed = {
      ...info,
      configured: false,
      host_key: "ssh-ed25519 discovered",
      host_key_fingerprint: "SHA256:" + "b".repeat(43),
      requires_host_key_confirmation: true,
    };
    mocked.save.mockResolvedValueOnce(observed).mockResolvedValueOnce({
      ...info,
      host_key: observed.host_key,
      host_key_fingerprint: observed.host_key_fingerprint,
    });
    mount({
      ...info,
      configured: false,
      host_key: "",
      host_key_fingerprint: "",
    });
    await user.click(
      screen.getByRole("button", { name: "Configure execution access" }),
    );
    await screen.findByDisplayValue("manager.example");
    expect(
      screen.queryByRole("textbox", { name: /public key|private key/i }),
    ).not.toBeInTheDocument();
    await user.type(screen.getByLabelText("SSH password"), "test-password");
    await user.click(screen.getByRole("button", { name: "Verify and save" }));
    await screen.findByRole("button", { name: "Trust and continue" });
    expect(mocked.save).toHaveBeenCalledTimes(1);
    expect(mocked.save).toHaveBeenLastCalledWith(
      "ws",
      "manager",
      7,
      expect.objectContaining({ host_key: "", password: "test-password" }),
    );
    expect(screen.getByText(observed.host_key_fingerprint)).toBeVisible();
    expect(screen.getByRole("dialog")).toBeVisible();
    await user.click(
      screen.getByRole("button", { name: "Trust and continue" }),
    );
    await waitFor(() =>
      expect(mocked.save).toHaveBeenLastCalledWith(
        "ws",
        "manager",
        7,
        expect.objectContaining({
          host_key: observed.host_key,
          password: "test-password",
        }),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });
  it("discards a pending server identity when the host changes", async () => {
    const user = userEvent.setup();
    mocked.save.mockResolvedValue({
      ...info,
      configured: false,
      host_key: "ssh-ed25519 discovered",
      requires_host_key_confirmation: true,
    });
    mount();
    await user.click(
      screen.getByRole("button", { name: "Configure execution access" }),
    );
    await screen.findByDisplayValue("manager.example");
    await user.click(
      screen.getByRole("button", { name: "Re-read fingerprint" }),
    );
    await screen.findByRole("button", { name: "Trust and continue" });
    await user.clear(screen.getByLabelText("SSH host"));
    await user.type(screen.getByLabelText("SSH host"), "other.example");
    await user.type(screen.getByLabelText("SSH password"), "new-host-password");
    expect(
      screen.queryByRole("button", { name: "Trust and continue" }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Verify and save" }));
    await waitFor(() =>
      expect(mocked.save).toHaveBeenLastCalledWith(
        "ws",
        "manager",
        7,
        expect.objectContaining({
          host: "other.example",
          host_key: "",
          probe_host_key: false,
        }),
      ),
    );
  });
  it("saves without asking the user to type a Kubernetes context", async () => {
    const user = userEvent.setup();
    mocked.save.mockResolvedValue({ ...info, context: "detected" });
    mount();
    await user.click(
      screen.getByRole("button", { name: "Configure execution access" }),
    );
    await screen.findByDisplayValue("manager.example");
    expect(
      screen.queryByRole("textbox", { name: /context/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("combobox", { name: "Cluster connection" }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Verify and save" }));
    await waitFor(() =>
      expect(mocked.save).toHaveBeenCalledWith(
        "ws",
        "manager",
        7,
        expect.objectContaining({
          context: "",
          password: "",
          host: "manager.example",
        }),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });
  it("keeps the dialog open and asks for a detected connection only when there are several", async () => {
    const user = userEvent.setup();
    mocked.save
      .mockResolvedValueOnce({
        ...info,
        configured: false,
        context: "",
        requires_context_selection: true,
        context_options: [
          { name: "prod", cluster: "logs" },
          { name: "qa", cluster: "test" },
        ],
      })
      .mockResolvedValueOnce({ ...info, context: "qa" });
    mount();
    await user.click(
      screen.getByRole("button", { name: "Configure execution access" }),
    );
    await screen.findByDisplayValue("manager.example");
    await user.click(screen.getByRole("button", { name: "Verify and save" }));
    const picker = await screen.findByRole("combobox", {
      name: "Cluster connection",
    });
    expect(screen.getByRole("dialog")).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Verify and save" }),
    ).toBeDisabled();
    await user.click(picker);
    await user.click(await screen.findByRole("option", { name: "qa · test" }));
    await user.click(screen.getByRole("button", { name: "Verify and save" }));
    await waitFor(() =>
      expect(mocked.save).toHaveBeenLastCalledWith(
        "ws",
        "manager",
        7,
        expect.objectContaining({ context: "qa" }),
      ),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
  });
});
