import { defineTool } from "eve/tools";
import { never } from "eve/tools/approval";
import { z } from "zod";

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export default defineTool({
  approval: never(),
  description: "Get the current weather for a city.",
  inputSchema: z.object({
    city: z.string(),
  }),
  async execute(input) {
    await sleep(300);

    return {
      city: input.city,
      temperatureF: 72,
      condition: "Sunny",
      summary: `Sunny in ${input.city} with a light breeze.`,
    };
  },
});
