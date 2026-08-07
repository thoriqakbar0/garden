import { defineTool } from "eve/tools";
import { never } from "eve/tools/approval";
import { z } from "zod";

export default defineTool({
  approval: never(),
  description: "Return deterministic authored TypeScript parity evidence.",
  inputSchema: z.object({ value: z.string() }),
  async execute({ value }) {
    return { marker: "authored-typescript", value };
  },
});
