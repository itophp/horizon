import { render, screen } from "@testing-library/react";
import { createMemoryRouter, RouterProvider } from "react-router";
import { expect, it, vi } from "vitest";
import { projectFrontendPlugin } from "./plugin";

vi.mock("./page", () => ({
  ProjectsPage: () => <h1>Project list fixture</h1>,
}));
vi.mock("./detail", () => ({
  ProjectDetailPage: () => <h1>Project detail fixture</h1>,
}));

it.each([
  ["/projects", "Project list fixture"],
  ["/projects/42", "Project detail fixture"],
])("loads the registered project route %s", async (path, heading) => {
  const router = createMemoryRouter(projectFrontendPlugin.routes, {
    initialEntries: [path],
  });
  render(<RouterProvider router={router} />);
  expect(
    await screen.findByRole("heading", { name: heading }),
  ).toBeInTheDocument();
});
