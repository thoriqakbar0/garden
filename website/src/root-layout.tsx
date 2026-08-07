import { lazy, Suspense } from "react";
import { Outlet } from "@tanstack/react-router";

const DevDialKitAuthoring = import.meta.env.DEV
  ? lazy(() => import("./dialkit-authoring"))
  : null;
const DevAgentation = import.meta.env.DEV
  ? lazy(() => import("agentation").then(({ Agentation }) => ({ default: Agentation })))
  : null;

export function RootLayout() {
  return (
    <>
      <Outlet />
      {DevDialKitAuthoring === null ? null : (
        <Suspense fallback={null}>
          <DevDialKitAuthoring />
        </Suspense>
      )}
      {DevAgentation === null ? null : (
        <Suspense fallback={null}>
          <DevAgentation />
        </Suspense>
      )}
    </>
  );
}
