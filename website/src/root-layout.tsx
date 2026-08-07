import { Outlet } from "@tanstack/react-router";
import { Agentation } from "agentation";

export function RootLayout() {
  return (
    <>
      <Outlet />
      {import.meta.env.DEV ? <Agentation /> : null}
    </>
  );
}
