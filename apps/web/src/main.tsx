import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { RouterProvider } from "react-router";
import { Providers } from "./app/providers/providers";
import { router } from "./app/router/router";
import "./styles/index.css";
import "./styles/admin.css";
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <Providers>
      <RouterProvider router={router} />
    </Providers>
  </StrictMode>,
);
