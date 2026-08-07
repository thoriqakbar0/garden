import { useState } from "react";

const resetDelayMilliseconds = 2_000;

type CopyCommandProps = Readonly<{
  command: string;
}>;

type CopyStatus = "idle" | "copied" | "failed";

function statusMessage(status: CopyStatus): string {
  switch (status) {
    case "idle":
      return "";
    case "copied":
      return "Command copied to clipboard";
    case "failed":
      return "Unable to copy. Select the command and copy it manually.";
  }
}

/** Copies an installation command and announces success or recovery guidance. */
export function CopyCommand({ command }: CopyCommandProps) {
  const [status, setStatus] = useState<CopyStatus>("idle");

  function copyToClipboard(): void {
    void navigator.clipboard.writeText(command).then(
      () => {
        setStatus("copied");
        window.setTimeout(() => setStatus("idle"), resetDelayMilliseconds);
      },
      () => {
        setStatus("failed");
      },
    );
  }

  return (
    <div className="copy-command">
      <code>{command}</code>
      <button type="button" onClick={copyToClipboard} aria-label={`Copy ${command}`}>
        <span aria-hidden="true">{status === "copied" ? "Copied" : "Copy"}</span>
      </button>
      <span className="sr-only" role="status" aria-live="polite">
        {statusMessage(status)}
      </span>
    </div>
  );
}
