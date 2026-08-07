import { defineAgent } from "eve";
import { mockModel } from "eve/evals";

const model = mockModel({
  modelId: "garden-official-parity",
  respond(request) {
    const bash = request.toolResults.find((result) => result.name === "bash");
    const authored = request.toolResults.find((result) => result.name === "typescript_echo");

    if (bash === undefined) {
      return {
        toolCalls: [{
          id: "garden-bash",
          name: "bash",
          input: { command: "printf sandbox-terminal-1to1" },
        }],
      };
    }

    const bashOutput = readBashOutput(bash.output);
    if (authored === undefined) {
      return {
        toolCalls: [{
          id: "garden-typescript",
          name: "typescript_echo",
          input: { value: `GARDEN-1TO1:${bashOutput.stdout}` },
        }],
      };
    }

    const authoredOutput = readAuthoredOutput(authored.output);
    return [
      `bash=${bashOutput.stdout}`,
      `authored=${authoredOutput.marker}`,
      `value=${authoredOutput.value}`,
    ].join("; ");
  },
});

export default defineAgent({ model, modelContextWindowTokens: 32_000 });

function readBashOutput(output: unknown): { exitCode: number; stdout: string } {
  if (!isRecord(output) || output.exitCode !== 0 || typeof output.stdout !== "string") {
    throw new Error("Bash parity output was not successful.");
  }
  return { exitCode: output.exitCode, stdout: output.stdout };
}

function readAuthoredOutput(output: unknown): { marker: string; value: string } {
  if (
    !isRecord(output) ||
    typeof output.marker !== "string" ||
    typeof output.value !== "string"
  ) {
    throw new Error("Authored parity output was malformed.");
  }
  return { marker: output.marker, value: output.value };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
