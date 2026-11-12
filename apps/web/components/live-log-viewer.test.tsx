import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LiveLogViewer } from "./live-log-viewer";

describe("LiveLogViewer", () => {
  it("renders lines and toggles auto-scroll", () => {
    render(
      <LiveLogViewer
        state="live"
        error={null}
        lines={[
          {
            seq: 1,
            deploymentId: "d",
            ts: "2026-11-11T00:00:00Z",
            source: "worker",
            level: "info",
            message: "cloning",
          },
        ]}
      />,
    );
    expect(screen.getByText("cloning")).toBeTruthy();
    expect(screen.getByText("Live")).toBeTruthy();
    const box = screen.getByLabelText("Auto-scroll") as HTMLInputElement;
    expect(box.checked).toBe(true);
    fireEvent.click(box);
    expect(box.checked).toBe(false);
  });
});
