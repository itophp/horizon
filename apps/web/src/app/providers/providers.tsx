import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { I18nextProvider } from "react-i18next";
import type { PropsWithChildren } from "react";
import { ThemeProvider } from "./theme";
import { i18n } from "@/i18n";
const client = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30000 },
    mutations: { retry: false },
  },
});
export function Providers({ children }: PropsWithChildren) {
  return (
    <I18nextProvider i18n={i18n}>
      <ThemeProvider>
        <QueryClientProvider client={client}>{children}</QueryClientProvider>
      </ThemeProvider>
    </I18nextProvider>
  );
}
