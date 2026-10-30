import {
  createContext,
  useContext,
  useLayoutEffect,
  useState,
  useCallback,
  type PropsWithChildren,
} from "react";
type Theme = "light" | "dark";
const key = "baseline.theme";
const ThemeContext = createContext<{
  theme: Theme;
  toggle: () => void;
  setTheme: (value: Theme) => void;
} | null>(null);
export function ThemeProvider({ children }: PropsWithChildren) {
  const [theme, updateTheme] = useState<Theme>(() =>
    localStorage.getItem(key) === "light" ? "light" : "dark",
  );
  const setTheme = useCallback((value: Theme) => updateTheme(value), []);
  useLayoutEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark");
    document.documentElement.style.colorScheme = theme;
  }, [theme]);
  return (
    <ThemeContext.Provider
      value={{
        theme,
        setTheme,
        toggle: () => {
          const value = theme === "dark" ? "light" : "dark";
          updateTheme(value);
          localStorage.setItem(key, value);
          window.dispatchEvent(
            new CustomEvent("baseline:preference", {
              detail: { key: "display.theme", value },
            }),
          );
        },
      }}
    >
      {children}
    </ThemeContext.Provider>
  );
}
export function useTheme() {
  const value = useContext(ThemeContext);
  if (!value) throw new Error("ThemeProvider is required");
  return value;
}
