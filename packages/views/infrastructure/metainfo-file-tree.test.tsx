import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { MetainfoFileTree } from "./metainfo-file-tree";

const files = [
  { key: "APP/v1/metainfo.yaml", name: "metainfo.yaml", type: "INFO_YAML" },
  {
    key: "APP/v1/i18n/service.properties",
    name: "i18n/service.properties",
    type: "I18N_FILE",
  },
  {
    key: "APP/v1/resources/images/icon.png",
    name: "resources/images/icon.png",
    type: "RES_FILE",
  },
  {
    key: "APP/v1/templates/config.yaml",
    name: "templates/config.yaml",
    type: "TPL_FILE",
  },
  {
    key: "APP/v1/templates/worker/config.yaml",
    name: "templates/worker/config.yaml",
    type: "TPL_FILE",
  },
];

describe("metainfo directory tree", () => {
  it("expands each directory level and keeps duplicate filenames in their own directories", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <MetainfoFileTree
        files={files}
        selectedKey={files[0]!.key}
        disabled={false}
        onSelect={onSelect}
      />,
    );
    expect(
      screen.getByRole("button", { name: "metainfo.yaml" }),
    ).toHaveAttribute("aria-current", "page");
    expect(
      screen.queryByRole("button", { name: "resources/images" }),
    ).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "resources" }));
    const images = screen.getByRole("button", {
      name: "resources/images",
    });
    expect(images).toHaveAttribute("aria-expanded", "false");
    await user.click(images);
    expect(
      within(images.closest("li")!).getByRole("button", {
        name: "resources/images/icon.png",
      }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "templates" }));
    const worker = screen.getByRole("button", {
      name: "templates/worker",
    });
    await user.click(worker);
    expect(
      within(worker.closest("li")!).getByRole("button", {
        name: "templates/worker/config.yaml",
      }),
    ).toHaveTextContent("config.yaml");
    expect(
      screen.getByRole("button", {
        name: "templates/config.yaml",
      }),
    ).toHaveTextContent("config.yaml");
    expect(onSelect).not.toHaveBeenCalled();
    await user.click(
      screen.getByRole("button", {
        name: "templates/worker/config.yaml",
      }),
    );
    expect(onSelect).toHaveBeenCalledWith(files[4]);
    await user.click(screen.getByRole("button", { name: "templates" }));
    expect(
      screen.queryByRole("button", {
        name: "templates/config.yaml",
      }),
    ).not.toBeInTheDocument();
  });

  it("reveals the selected file and reopens its ancestors when selection changes", async () => {
    const user = userEvent.setup();
    const props = {
      files,
      selectedKey: files[4]!.key,
      disabled: false,
      onSelect: vi.fn(),
    };
    const { rerender } = render(<MetainfoFileTree {...props} />);
    expect(
      screen.getByRole("button", { name: "templates/worker/config.yaml" }),
    ).toHaveAttribute("aria-current", "page");
    await user.click(screen.getByRole("button", { name: "templates" }));
    expect(
      screen.queryByRole("button", { name: "templates/worker/config.yaml" }),
    ).not.toBeInTheDocument();
    rerender(<MetainfoFileTree {...props} selectedKey={files[1]!.key} />);
    expect(
      await screen.findByRole("button", { name: "i18n/service.properties" }),
    ).toHaveAttribute("aria-current", "page");
    rerender(<MetainfoFileTree {...props} />);
    expect(
      await screen.findByRole("button", {
        name: "templates/worker/config.yaml",
      }),
    ).toHaveAttribute("aria-current", "page");
  });

  it("allows directory browsing while another file has unsaved changes", async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <MetainfoFileTree
        files={files}
        selectedKey={files[0]!.key}
        disabled
        onSelect={onSelect}
      />,
    );
    const directory = screen.getByRole("button", { name: "i18n" });
    expect(directory).toBeEnabled();
    await user.click(directory);
    expect(
      screen.getByRole("button", { name: "i18n/service.properties" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "metainfo.yaml" })).toBeEnabled();
    expect(onSelect).not.toHaveBeenCalled();
  });
});
