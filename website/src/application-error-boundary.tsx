import { Component, type ErrorInfo, type ReactNode } from "react";

interface ApplicationErrorBoundaryProps {
  children: ReactNode;
}

interface ApplicationErrorBoundaryState {
  hasError: boolean;
}

export class ApplicationErrorBoundary extends Component<
  ApplicationErrorBoundaryProps,
  ApplicationErrorBoundaryState
> {
  public state: ApplicationErrorBoundaryState = { hasError: false };

  public static getDerivedStateFromError(): ApplicationErrorBoundaryState {
    return { hasError: true };
  }

  public componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("Garden failed to render.", error, info);
  }

  public render(): ReactNode {
    if (this.state.hasError) {
      return <ApplicationFallback />;
    }

    return this.props.children;
  }
}

function ApplicationFallback() {
  return (
    <main className="boot-fallback mx-auto flex min-h-screen w-full max-w-[52rem] flex-col justify-center px-6 py-16">
      <div className="mb-8 inline-flex items-center gap-2.5 font-display text-[1.1rem] font-bold tracking-[-0.025em]">
        <span className="size-3 rounded-full bg-leaf" aria-hidden="true" />
        garden
      </div>
      <h1 className="m-0 max-w-[12ch] font-display text-[clamp(3rem,8vw,5.5rem)] font-[650] leading-[0.95] tracking-[-0.04em] text-ink text-balance">
        Garden did not load.
      </h1>
      <p className="mt-7 max-w-[38rem] text-[1.05rem] leading-[1.65] text-ink-soft text-pretty">
        Reload the page to try again. If it still fails, Garden’s source and
        documentation remain available on GitHub.
      </p>
      <div className="mt-8 flex flex-wrap gap-3">
        <a className="inline-flex min-h-13 items-center justify-center rounded-[var(--radius-sm)] bg-forest px-5 text-sm font-[750] text-paper-bright no-underline" href="/">
          Reload Garden
        </a>
        <a className="inline-flex min-h-13 items-center justify-center rounded-[var(--radius-sm)] px-5 text-sm font-[750] text-forest shadow-[inset_0_0_0_1px_var(--line-strong)]" href="https://github.com/thoriqakbar0/garden">
          Open Garden on GitHub
        </a>
      </div>
    </main>
  );
}
