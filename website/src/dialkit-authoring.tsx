import { useEffect } from "react";
import { DialRoot, useDialKit } from "dialkit";
import "dialkit/styles.css";

import { setDitherScale } from "./dither-context";

export default function DialKitAuthoring() {
  const dither = useDialKit("Dither", {
    ditherScale: [2, 1, 12, 1],
  }, {
    id: "garden-dither",
    persist: true,
  });

  useEffect(() => {
    setDitherScale(dither.ditherScale);
  }, [dither.ditherScale]);

  return <DialRoot position="top-right" />;
}
