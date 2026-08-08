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
    <div className="grid min-h-15 grid-cols-[minmax(0,1fr)_auto] items-center gap-4 px-6 font-mono max-[31rem]:grid-cols-1 max-[31rem]:px-4 max-[31rem]:py-3">
      <code className="overflow-wrap-anywhere text-xs">{command}</code>
      <button className="min-h-10 min-w-14 cursor-pointer rounded-[0.45rem] border border-[oklch(0.82_0.155_116/0.32)] bg-transparent px-3 font-body text-[0.65rem] font-[650] text-leaf transition-[color,background-color,transform] duration-150 hover:bg-leaf hover:text-forest-deep active:scale-96 max-[31rem]:w-full" type="button" onClick={copyToClipboard} aria-label={`Copy ${command}`}>
        <span aria-hidden="true">{status === "copied" ? "Copied" : "Copy"}</span>
      </button>
      <span className="absolute -m-px size-px overflow-hidden border-0 p-0 whitespace-nowrap [clip:rect(0,0,0,0)]" role="status" aria-live="polite">
        {statusMessage(status)}
      </span>
    </div>
  );
}
