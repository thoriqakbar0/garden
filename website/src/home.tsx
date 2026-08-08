import { useEffect, useState } from "react";

import { CopyCommand } from "./copy-command";
import { useDitherScale } from "./dither-context";
import { DitheredField, DitheredTrees } from "./dithered-field";

const githubUrl = "https://github.com/thoriqakbar0/garden";

/* ─────────────────────────────────────────────────────────
 * HERO ANIMATION STORYBOARD
 *
 *    0ms  hero waits in its composed layout
 *  100ms  headline and supporting copy settle into view
 * ───────────────────────────────────────────────────────── */

const TIMING = {
  heroCopy: 100,
} as const;

const finalHeroStage = 1;

function revealClassName(baseClassName: string, isRevealed: boolean): string {
  return isRevealed ? `${baseClassName} is-revealed` : baseClassName;
}

function useHeroStage(): number {
  const prefersReducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  const [stage, setStage] = useState(prefersReducedMotion ? finalHeroStage : 0);

  useEffect(() => {
    if (prefersReducedMotion) {
      return;
    }

    const timers = Object.values(TIMING).map((delay, index) =>
      window.setTimeout(() => setStage(index + 1), delay),
    );

    return () => timers.forEach((timer) => window.clearTimeout(timer));
  }, [prefersReducedMotion]);

  return stage;
}

function GardenMark() {
  return (
    <svg viewBox="0 0 32 32" aria-hidden="true" focusable="false">
      <path d="M16 27V12" />
      <path d="M16 16C10 16 6 12 6 6c6 0 10 4 10 10Z" />
      <path d="M16 21c0-6 4-10 10-10 0 6-4 10-10 10Z" />
    </svg>
  );
}

function ArrowIcon() {
  return (
    <svg className="w-[1.05rem] fill-none stroke-current [stroke-linecap:round] [stroke-linejoin:round] [stroke-width:2]" viewBox="0 0 20 20" aria-hidden="true" focusable="false">
      <path d="M4 10h11M11 6l4 4-4 4" />
    </svg>
  );
}

function ExternalLinkCue() {
  return (
    <>
      <svg className="w-[1.05rem] fill-none stroke-current [stroke-linecap:round] [stroke-linejoin:round] [stroke-width:2]" viewBox="0 0 20 20" aria-hidden="true" focusable="false">
        <path d="M8 5H5v10h10v-3M11 5h4v4M15 5l-7 7" />
      </svg>
      <span className="sr-only"> (opens in a new tab)</span>
    </>
  );
}

/** Renders Garden’s primary marketing page. */
export function Home() {
  const ditherScale = useDitherScale();
  const heroStage = useHeroStage();

  return (
    <>
      <a className="fixed start-3 top-3 z-[100] -translate-y-[200%] rounded-[var(--radius-sm)] bg-forest-deep px-4 py-3 text-paper-bright focus:translate-y-0" href="#main-content">
        Skip to content
      </a>

      <header className="absolute inset-x-0 top-0 z-20 py-5 max-[48rem]:py-3.5">
        <nav className="mx-auto flex min-h-13 w-shell items-center justify-between gap-8 max-[48rem]:gap-4" aria-label="Main navigation">
          <a className="inline-flex min-h-11 items-center gap-2.5 font-display text-[1.1rem] font-bold tracking-[-0.025em] no-underline" href="#top" aria-label="Garden home">
            <span className="brand-mark grid size-8 -rotate-6 place-items-center rounded-[50%_50%_45%_55%] bg-leaf text-forest-deep"><GardenMark /></span>
            <span>garden</span>
          </a>

          <div className="ms-auto flex items-center gap-8 max-[48rem]:hidden">
            <a href="#runtime">How it works</a>
            <a href="#protocol">Compatibility</a>
            <a href="#install">Install</a>
          </div>

          <a className="ms-auto inline-flex min-h-11 items-center gap-1.5 text-sm font-[650] text-forest no-underline transition-colors duration-150 hover:text-forest-deep max-[48rem]:min-w-11 max-[48rem]:justify-center max-[31rem]:gap-0 max-[31rem]:text-[0]" href={githubUrl} target="_blank" rel="noreferrer">
            GitHub <ExternalLinkCue />
          </a>
        </nav>
      </header>

      <main id="main-content">
        <section className="hero relative min-h-[50rem] overflow-hidden bg-[radial-gradient(circle_at_78%_23%,oklch(0.82_0.155_116/0.18),transparent_20rem),linear-gradient(130deg,var(--paper-bright),var(--paper)_58%,var(--paper-deep))] py-[9.5rem] pb-[6.5rem] max-[64rem]:min-h-0 max-[48rem]:py-32 max-[48rem]:pb-18" id="top">
          <DitheredField ditherScale={ditherScale} />
          <div className="relative z-[1] mx-auto grid w-shell grid-cols-[minmax(0,44rem)] items-center max-[64rem]:grid-cols-1 max-[48rem]:gap-y-0">
            <div className={revealClassName("hero-copy max-w-[44rem]", heroStage >= 1)}>
              <h1 className="m-0 max-w-[10ch] font-display text-[clamp(4rem,7.6vw,6rem)] font-[650] leading-[0.95] tracking-[-0.04em] text-balance max-[64rem]:max-w-[11ch] max-[48rem]:max-w-[12ch] max-[48rem]:text-[clamp(2.9rem,12.8vw,3.45rem)]">
                <span className="block text-ink">Run Eve on your</span>
                <span className="block text-forest">infrastructure.</span>
              </h1>
              <p className="mt-8 max-w-[38rem] text-[clamp(1.05rem,1.7vw,1.25rem)] leading-[1.6] text-ink-soft text-pretty max-[48rem]:mt-[1.1rem] max-[48rem]:text-base">
                Garden is a self-hosted Go runtime for Eve-compatible agents. It
                can also launch and supervise Eve by Vercel when you need the
                complete framework.
              </p>

              <div className="mt-8 flex flex-wrap items-center gap-3.5 max-[48rem]:mt-[1.15rem] max-[31rem]:flex-nowrap max-[31rem]:gap-2.5">
                <a className="button inline-flex min-h-13 items-center justify-center gap-2 rounded-[var(--radius-sm)] bg-forest px-5 text-sm font-[750] text-paper-bright no-underline shadow-[0_0.45rem_0.9rem_oklch(0.24_0.08_148/0.18),0_0.1rem_0.25rem_oklch(0.24_0.08_148/0.2)] transition-[color,background-color,box-shadow,transform] duration-160 ease-[cubic-bezier(0.2,0,0,1)] hover:bg-forest-deep hover:shadow-[0_0.6rem_1.15rem_oklch(0.24_0.08_148/0.22),0_0.1rem_0.25rem_oklch(0.24_0.08_148/0.2)] active:scale-96 max-[31rem]:min-w-0 max-[31rem]:flex-1 max-[31rem]:px-2.5 max-[31rem]:text-xs" href="#install">
                  Install Garden <ArrowIcon />
                </a>
                <a className="button inline-flex min-h-13 items-center justify-center gap-2 rounded-[var(--radius-sm)] px-5 text-sm font-[750] text-forest no-underline shadow-[inset_0_0_0_1px_var(--line-strong)] transition-[color,background-color,box-shadow,transform] duration-160 ease-[cubic-bezier(0.2,0,0,1)] hover:bg-paper-bright active:scale-96 max-[31rem]:min-w-0 max-[31rem]:flex-1 max-[31rem]:px-2.5 max-[31rem]:text-xs" href="https://github.com/thoriqakbar0/garden/blob/main/COMPATIBILITY.md" target="_blank" rel="noreferrer">
                  Review compatibility <ExternalLinkCue />
                </a>
              </div>

              <dl className="m-0 mt-12 flex flex-wrap gap-x-8 gap-y-5 max-[48rem]:mt-6 max-[48rem]:gap-x-6 max-[48rem]:gap-y-4 max-[31rem]:grid max-[31rem]:grid-cols-2" aria-label="Garden operating model">
                <div className="grid gap-0.5"><dt className="font-mono text-xs font-semibold uppercase leading-[1.4] tracking-[0.08em] text-ink-soft">Runs</dt><dd className="m-0 text-sm font-bold text-ink">On your infrastructure</dd></div>
                <div className="grid gap-0.5"><dt className="font-mono text-xs font-semibold uppercase leading-[1.4] tracking-[0.08em] text-ink-soft">Stores</dt><dd className="m-0 text-sm font-bold text-ink">Local workflow history</dd></div>
                <div className="grid gap-0.5"><dt className="font-mono text-xs font-semibold uppercase leading-[1.4] tracking-[0.08em] text-ink-soft">Requires</dt><dd className="m-0 text-sm font-bold text-ink">No hosted Garden service</dd></div>
              </dl>
            </div>

          </div>
        </section>

        <section className="runtime-section relative overflow-hidden py-[clamp(6rem,10vw,9rem)]" id="runtime">
          <div className="relative z-[1] mx-auto w-shell">
            <div className="grid grid-cols-[minmax(0,1.05fr)_minmax(18rem,0.65fr)] items-end gap-12 max-[48rem]:grid-cols-1">
              <h2 className="m-0 max-w-[16ch] font-display text-[clamp(2.7rem,5.3vw,4.8rem)] font-[650] leading-[1.02] tracking-[-0.04em] text-balance">One Go process. Nothing hosted in the middle.</h2>
              <p className="m-0 max-w-[38rem] text-[1.05rem] text-ink-soft text-pretty">
                Point Garden at an Eve-shaped project and run the supported
                contract locally. Garden owns the server, workflow history, and
                model loop; your infrastructure owns the process.
              </p>
            </div>

            <div className="runtime-landscape relative mt-18 grid grid-cols-[1fr_4.5rem_1fr] items-stretch max-[48rem]:grid-cols-1 max-[48rem]:gap-4">
              <article className="mode relative min-h-[30rem] rounded-[var(--radius-lg)] bg-paper-deep p-[clamp(1.6rem,3vw,2.6rem)] text-ink max-[48rem]:min-h-0">
                <h3 className="mb-5 max-w-[13ch] font-display text-[clamp(2rem,3.2vw,3rem)] leading-[1.05] tracking-[-0.035em] text-balance">Run Garden natively.</h3>
                <div className="mode-topline mb-4 flex justify-between gap-4">
                  <span className="font-mono text-xs font-semibold uppercase leading-[1.4] tracking-[0.08em] text-ink-soft">Self-hosted Go runtime</span><code className="font-mono text-xs">garden</code>
                </div>
                <p className="m-0 max-w-[34rem] text-ink-soft text-pretty">
                  Garden runs sessions, streams, model and native-tool turns,
                  cancellation, and recovery without a hosted Garden service. It
                  does not execute arbitrary authored TypeScript.
                </p>
                <div className="mt-8 overflow-x-auto rounded-[var(--radius-sm)] bg-[oklch(0.99_0.007_96/0.72)] p-4 font-mono text-xs"><code>garden serve</code></div>
                <ul className="mt-8 grid list-none gap-3 p-0 text-sm text-ink-soft" aria-label="Garden characteristics">
                  <li className="mode-item flex items-center gap-2.5">OpenAI, Anthropic, Google, or Codex</li>
                  <li className="mode-item flex items-center gap-2.5">Fsync-backed local workflow history</li>
                  <li className="mode-item flex items-center gap-2.5">Loopback by default</li>
                </ul>
              </article>

              <div className="mode-divider relative grid place-items-center max-[48rem]:min-h-14" aria-hidden="true">
                <span className="relative z-[1] bg-paper px-0 py-3 font-mono text-xs uppercase tracking-[0.08em] text-ink-soft [writing-mode:vertical-rl] max-[48rem]:px-3 max-[48rem]:py-0 max-[48rem]:[writing-mode:horizontal-tb]">same project shape</span>
              </div>

              <article className="mode relative min-h-[30rem] rounded-[var(--radius-lg)] bg-forest p-[clamp(1.6rem,3vw,2.6rem)] text-paper-bright shadow-[0_1.5rem_4rem_oklch(0.22_0.07_148/0.2)] max-[48rem]:min-h-0">
                <h3 className="mb-5 max-w-[13ch] font-display text-[clamp(2rem,3.2vw,3rem)] leading-[1.05] tracking-[-0.035em] text-balance">Run Eve in full.</h3>
                <div className="mode-topline mb-4 flex justify-between gap-4">
                  <span className="font-mono text-xs font-semibold uppercase leading-[1.4] tracking-[0.08em] text-[oklch(0.94_0.025_112/0.8)]">Eve by Vercel</span><code className="font-mono text-xs text-[oklch(0.94_0.025_112/0.8)]">eve@0.27.6</code>
                </div>
                <p className="m-0 max-w-[34rem] text-[oklch(0.94_0.025_112/0.84)] text-pretty">
                  Eve by Vercel is the definitive experience: authored TypeScript,
                  tools, sessions, protocol behavior, and sandboxing working as one
                  system. Garden can validate and supervise your pinned project-local
                  Eve process while Eve remains fully in control.
                </p>
                <div className="mt-8 overflow-x-auto rounded-[var(--radius-sm)] bg-[oklch(0.17_0.04_150/0.23)] p-4 font-mono text-xs"><code>garden serve --runtime eve</code></div>
                <ul className="mt-8 grid list-none gap-3 p-0 text-sm text-[oklch(0.94_0.025_112/0.8)]" aria-label="Eve by Vercel characteristics">
                  <li className="mode-item flex items-center gap-2.5">Run the complete authored feature set</li>
                  <li className="mode-item flex items-center gap-2.5">Execute project TypeScript unchanged</li>
                  <li className="mode-item flex items-center gap-2.5">Keep Eve in control of runtime behavior</li>
                </ul>
              </article>
            </div>
          </div>
        </section>

        <section className="protocol-section relative overflow-hidden bg-forest-deep text-paper-bright" id="protocol">
          <div className="relative z-[1] mx-auto grid w-shell grid-cols-[minmax(0,0.8fr)_minmax(28rem,1.2fr)] items-center gap-[clamp(3rem,8vw,8rem)] py-[clamp(6rem,10vw,9rem)] max-[64rem]:grid-cols-1">
            <div className="protocol-copy max-[64rem]:max-w-[42rem]">
              <h2 className="m-0 max-w-[12ch] font-display text-[clamp(2.7rem,5.3vw,4.8rem)] font-[650] leading-[1.02] tracking-[-0.04em] text-balance">Know exactly what runs.</h2>
              <p className="mt-7 max-w-[38rem] text-[1.05rem] text-[oklch(0.91_0.025_110/0.75)] text-pretty">
                Garden documents native support, partial behavior, discovery-only
                features, and official Eve delegation separately. The compatibility
                matrix ties each claim to its evidence instead of promising broad parity.
              </p>
              <a className="mt-7 inline-flex min-h-11 items-center gap-2 text-sm font-[750] text-leaf no-underline" href="https://github.com/thoriqakbar0/garden/blob/main/COMPATIBILITY.md" target="_blank" rel="noreferrer">
                Review the compatibility matrix <ExternalLinkCue />
              </a>
            </div>

            <figure className="event-field m-0 overflow-hidden rounded-[var(--radius-lg)] bg-[oklch(0.16_0.035_150/0.8)] shadow-[0_1.5rem_4rem_oklch(0.08_0.025_150/0.35),inset_0_0_0_1px_oklch(0.9_0.05_110/0.1)] max-[64rem]:w-[min(100%,42rem)]">
              <figcaption className="absolute -m-px size-px overflow-hidden border-0 p-0 whitespace-nowrap [clip:rect(0,0,0,0)]">Example Garden event stream</figcaption>
              <div className="event-field-head flex min-h-16 items-center justify-between border-b border-[oklch(0.9_0.05_110/0.1)] px-6 font-mono text-xs font-semibold uppercase leading-[1.4] tracking-[0.08em] text-[oklch(0.9_0.03_110/0.6)]">
                <span>session stream</span>
                <span><i /> live</span>
              </div>
              <ol className="m-0 list-none p-0">
                <li className="grid min-h-16 grid-cols-[2.6rem_minmax(0,1fr)_auto] items-center gap-4 border-b border-[oklch(0.9_0.05_110/0.08)] px-[1.4rem] font-mono max-[48rem]:grid-cols-[2rem_minmax(0,1fr)] max-[48rem]:gap-x-3 max-[48rem]:gap-y-1 max-[48rem]:py-3.5"><span className="text-xs text-[oklch(0.92_0.03_110/0.7)] tabular-nums">001</span><code className="text-xs text-[oklch(0.94_0.025_110)]">session.started</code><small className="text-xs text-[oklch(0.92_0.03_110/0.72)] [overflow-wrap:anywhere] max-[48rem]:col-start-2">ses_yk7</small></li>
                <li className="grid min-h-16 grid-cols-[2.6rem_minmax(0,1fr)_auto] items-center gap-4 border-b border-[oklch(0.9_0.05_110/0.08)] px-[1.4rem] font-mono max-[48rem]:grid-cols-[2rem_minmax(0,1fr)] max-[48rem]:gap-x-3 max-[48rem]:gap-y-1 max-[48rem]:py-3.5"><span className="text-xs text-[oklch(0.92_0.03_110/0.7)] tabular-nums">002</span><code className="text-xs text-[oklch(0.94_0.025_110)]">turn.started</code><small className="text-xs text-[oklch(0.92_0.03_110/0.72)] [overflow-wrap:anywhere] max-[48rem]:col-start-2">turn_01</small></li>
                <li className="grid min-h-16 grid-cols-[2.6rem_minmax(0,1fr)_auto] items-center gap-4 border-b border-[oklch(0.9_0.05_110/0.08)] px-[1.4rem] font-mono max-[48rem]:grid-cols-[2rem_minmax(0,1fr)] max-[48rem]:gap-x-3 max-[48rem]:gap-y-1 max-[48rem]:py-3.5"><span className="text-xs text-[oklch(0.92_0.03_110/0.7)] tabular-nums">003</span><code className="text-xs text-[oklch(0.94_0.025_110)]">message.received</code><small className="text-xs text-[oklch(0.92_0.03_110/0.72)] [overflow-wrap:anywhere] max-[48rem]:col-start-2">user</small></li>
                <li className="grid min-h-16 grid-cols-[2.6rem_minmax(0,1fr)_auto] items-center gap-4 border-b border-[oklch(0.9_0.05_110/0.08)] px-[1.4rem] font-mono max-[48rem]:grid-cols-[2rem_minmax(0,1fr)] max-[48rem]:gap-x-3 max-[48rem]:gap-y-1 max-[48rem]:py-3.5"><span className="text-xs text-[oklch(0.92_0.03_110/0.7)] tabular-nums">004</span><code className="text-xs text-[oklch(0.94_0.025_110)]">step.started</code><small className="text-xs text-[oklch(0.92_0.03_110/0.72)] [overflow-wrap:anywhere] max-[48rem]:col-start-2">model</small></li>
                <li className="bg-[oklch(0.82_0.155_116/0.1)] grid min-h-16 grid-cols-[2.6rem_minmax(0,1fr)_auto] items-center gap-4 border-b border-[oklch(0.9_0.05_110/0.08)] px-[1.4rem] font-mono max-[48rem]:grid-cols-[2rem_minmax(0,1fr)] max-[48rem]:gap-x-3 max-[48rem]:gap-y-1 max-[48rem]:py-3.5"><span className="text-xs text-[oklch(0.92_0.03_110/0.7)] tabular-nums">005</span><code className="text-xs text-leaf">message.completed</code><small className="text-xs text-leaf [overflow-wrap:anywhere] max-[48rem]:col-start-2">assistant</small></li>
                <li className="border-b-0 grid min-h-16 grid-cols-[2.6rem_minmax(0,1fr)_auto] items-center gap-4 border-b border-[oklch(0.9_0.05_110/0.08)] px-[1.4rem] font-mono max-[48rem]:grid-cols-[2rem_minmax(0,1fr)] max-[48rem]:gap-x-3 max-[48rem]:gap-y-1 max-[48rem]:py-3.5"><span className="text-xs text-[oklch(0.92_0.03_110/0.7)] tabular-nums">006</span><code className="text-xs text-[oklch(0.94_0.025_110)]">session.waiting</code><small className="text-xs text-[oklch(0.92_0.03_110/0.72)] [overflow-wrap:anywhere] max-[48rem]:col-start-2">continuable</small></li>
              </ol>
            </figure>
          </div>
        </section>

        <section className="relative overflow-hidden py-[clamp(6rem,10vw,9rem)]" aria-labelledby="principles-title">
          <div className="relative z-[1] mx-auto w-shell">
            <div className="grid grid-cols-[minmax(16rem,0.72fr)_minmax(24rem,1.28fr)] items-center gap-[clamp(3rem,8vw,8rem)] max-[48rem]:grid-cols-1">
              <h2 className="m-0 max-w-[12ch] font-display text-[clamp(2.5rem,4.5vw,4rem)] font-[650] leading-[1.02] tracking-[-0.04em] text-balance" id="principles-title">Local by default. Explicit by design.</h2>
              <DitheredTrees ditherScale={ditherScale} />
            </div>
            <div className="mt-[clamp(3rem,6vw,5rem)] grid">
              <article className="grid grid-cols-[1.5rem_minmax(10rem,0.45fr)_minmax(0,1fr)] items-start gap-5 border-t border-line py-7 max-[48rem]:grid-cols-[1.2rem_1fr]">
                <span className="mt-1 h-[1.1rem] w-3 rotate-12 rounded-[80%_15%_80%_15%] bg-leaf" aria-hidden="true" />
                <h3 className="m-0 font-display text-[1.1rem] tracking-[-0.02em]">Control exposure</h3>
                <p className="m-0 max-w-[50ch] text-sm text-ink-soft text-pretty max-[48rem]:col-start-2">Garden binds to loopback by default and requires bearer authentication beyond it.</p>
              </article>
              <article className="grid grid-cols-[1.5rem_minmax(10rem,0.45fr)_minmax(0,1fr)] items-start gap-5 border-t border-line py-7 max-[48rem]:grid-cols-[1.2rem_1fr]">
                <span className="mt-1 h-[1.1rem] w-3 rotate-12 rounded-[80%_15%_80%_15%] bg-leaf" aria-hidden="true" />
                <h3 className="m-0 font-display text-[1.1rem] tracking-[-0.02em]">Keep durable history</h3>
                <p className="m-0 max-w-[50ch] text-sm text-ink-soft text-pretty max-[48rem]:col-start-2">Fsync-backed events preserve session history and support deterministic local recovery.</p>
              </article>
              <article className="grid grid-cols-[1.5rem_minmax(10rem,0.45fr)_minmax(0,1fr)] items-start gap-5 border-y border-line py-7 max-[48rem]:grid-cols-[1.2rem_1fr]">
                <span className="mt-1 h-[1.1rem] w-3 rotate-12 rounded-[80%_15%_80%_15%] bg-leaf" aria-hidden="true" />
                <h3 className="m-0 font-display text-[1.1rem] tracking-[-0.02em]">Verify the boundary</h3>
                <p className="m-0 max-w-[50ch] text-sm text-ink-soft text-pretty max-[48rem]:col-start-2">Executable tests distinguish native support from partial behavior, delegation, discovery, and unsupported features.</p>
              </article>
            </div>
          </div>
        </section>

        <section className="relative overflow-hidden bg-paper-deep py-[clamp(5rem,9vw,8rem)]" id="install">
          <div className="relative z-[1] mx-auto grid w-shell grid-cols-[minmax(0,0.85fr)_minmax(30rem,1.15fr)] items-center gap-[clamp(3rem,8vw,8rem)] max-[64rem]:grid-cols-1">
            <div className="install-copy">
              <h2 className="m-0 max-w-[12ch] font-display text-[clamp(2.7rem,5.3vw,4.8rem)] font-[650] leading-[1.02] tracking-[-0.04em] text-balance">Run Garden on your machine.</h2>
              <p className="mt-6 max-w-[38rem] text-[1.05rem] text-ink-soft text-pretty">
                Install one Go binary, point it at an Eve-shaped project, and
                start the local server. No account or hosted Garden service.
              </p>
              <div className="mt-6 flex flex-wrap gap-2.5">
                <span className="rounded-full bg-[oklch(0.99_0.007_96/0.65)] px-2.5 py-1.5 font-mono text-xs text-ink-soft">macOS or Linux</span>
                <span className="rounded-full bg-[oklch(0.99_0.007_96/0.65)] px-2.5 py-1.5 font-mono text-xs text-ink-soft">Go 1.25+</span>
                <span className="rounded-full bg-[oklch(0.99_0.007_96/0.65)] px-2.5 py-1.5 font-mono text-xs text-ink-soft">Node 24+ only for Eve mode</span>
              </div>
            </div>

            <div className="overflow-hidden rounded-[var(--radius-lg)] bg-forest-deep text-[oklch(0.92_0.025_110)] shadow-[0_1.5rem_4rem_oklch(0.22_0.06_148/0.2)] max-[64rem]:w-[min(100%,42rem)] max-[31rem]:rounded-[var(--radius-md)]">
              <div className="flex min-h-16 items-center justify-between border-b border-[oklch(0.9_0.05_110/0.1)] px-6 font-mono text-xs font-semibold uppercase leading-[1.4] tracking-[0.08em] text-[oklch(0.9_0.03_110/0.55)]"><span>Install, then run Garden</span><span>shell</span></div>
              <div className="border-b border-[oklch(0.9_0.05_110/0.1)]">
                <span className="flex items-center gap-2.5 px-6 pt-4 pb-1 font-mono text-xs font-[650] uppercase tracking-[0.055em] text-[oklch(0.92_0.025_110/0.72)]"><i className="grid size-[1.35rem] place-items-center rounded-full bg-leaf not-italic text-forest-deep">1</i> Install Garden</span>
                <CopyCommand command="git clone https://github.com/thoriqakbar0/garden.git && cd garden && make install" />
              </div>
              <div className="border-b border-[oklch(0.9_0.05_110/0.1)]">
                <span className="flex items-center gap-2.5 px-6 pt-4 pb-1 font-mono text-xs font-[650] uppercase tracking-[0.055em] text-[oklch(0.92_0.025_110/0.72)]"><i className="grid size-[1.35rem] place-items-center rounded-full bg-leaf not-italic text-forest-deep">2</i> Run Garden</span>
                <CopyCommand command="garden serve --root /path/to/eve-project" />
              </div>
              <div className="grid min-h-16 grid-cols-[auto_minmax(0,1fr)] items-center gap-4 px-6 font-mono text-xs text-[oklch(0.9_0.03_110/0.55)] max-[31rem]:px-4"><span className="text-leaf">[live]</span><code className="text-end">http://127.0.0.1:3000</code></div>
            </div>
          </div>
        </section>

        <section className="relative overflow-hidden py-[clamp(7rem,12vw,11rem)] text-center">
          <div className="relative z-[1] mx-auto grid w-shell justify-items-center">
            <div className="closing-mark mb-8 grid size-16 -rotate-6 place-items-center rounded-[50%_50%_45%_55%] bg-leaf text-forest" aria-hidden="true"><GardenMark /></div>
            <h2 className="m-0 max-w-[17ch] font-display text-[clamp(2.7rem,5.3vw,4.8rem)] font-[650] leading-[1.02] tracking-[-0.04em] text-balance">Own a runtime you can inspect.</h2>
            <div className="mt-8 flex flex-wrap items-center justify-center gap-3.5 max-[31rem]:flex-col max-[31rem]:items-stretch">
              <a className="button inline-flex min-h-13 items-center justify-center gap-2 rounded-[var(--radius-sm)] bg-forest px-5 text-sm font-[750] text-paper-bright no-underline shadow-[0_0.45rem_0.9rem_oklch(0.24_0.08_148/0.18),0_0.1rem_0.25rem_oklch(0.24_0.08_148/0.2)] transition-[color,background-color,box-shadow,transform] duration-160 ease-[cubic-bezier(0.2,0,0,1)] hover:bg-forest-deep hover:shadow-[0_0.6rem_1.15rem_oklch(0.24_0.08_148/0.22),0_0.1rem_0.25rem_oklch(0.24_0.08_148/0.2)] active:scale-96 max-[31rem]:min-w-0 max-[31rem]:flex-1 max-[31rem]:px-2.5 max-[31rem]:text-xs" href={githubUrl} target="_blank" rel="noreferrer">
                Get Garden on GitHub <ExternalLinkCue />
              </a>
              <a className="inline-flex min-h-13 items-center gap-2 px-3 text-sm font-[750] text-forest" href="https://github.com/thoriqakbar0/garden/blob/main/README.md" target="_blank" rel="noreferrer">
                Read the Garden documentation <ExternalLinkCue />
              </a>
            </div>
          </div>
        </section>
      </main>

      <footer className="bg-forest-deep text-[oklch(0.9_0.03_110/0.75)]">
        <div className="mx-auto grid min-h-36 w-shell grid-cols-[1fr_auto_1fr] items-center gap-8 py-0 max-[48rem]:grid-cols-1 max-[48rem]:gap-4 max-[48rem]:py-8">
          <a className="inline-flex min-h-11 items-center gap-2.5 font-display text-[1.1rem] font-bold tracking-[-0.025em] no-underline" href="#top" aria-label="Back to Garden home">
            <span className="brand-mark grid size-8 -rotate-6 place-items-center rounded-[50%_50%_45%_55%] bg-forest text-paper-bright"><GardenMark /></span>
            <span>garden</span>
          </a>
          <p className="footer-attribution grid max-w-[29rem] text-center text-xs leading-[1.55] max-[48rem]:text-start">
            <span className="font-[650] text-paper-bright">Garden is an independent implementation of a supported Eve-compatible subset.</span>
            <span>Eve is the complete framework and runtime by Vercel.</span>
          </p>
          <div className="flex justify-end gap-6 max-[48rem]:flex-wrap max-[48rem]:justify-start">
            <a className="inline-flex min-h-11 items-center gap-1.5 text-xs font-[650]" href="https://github.com/thoriqakbar0/garden/blob/main/COMPATIBILITY.md" target="_blank" rel="noreferrer">Compatibility <ExternalLinkCue /></a>
            <a className="inline-flex min-h-11 items-center gap-1.5 text-xs font-[650]" href="https://github.com/thoriqakbar0/garden/blob/main/THIRD_PARTY_NOTICES.md" target="_blank" rel="noreferrer">Licenses <ExternalLinkCue /></a>
            <a className="inline-flex min-h-11 items-center gap-1.5 text-xs font-[650]" href={githubUrl} target="_blank" rel="noreferrer">Source <ExternalLinkCue /></a>
          </div>
        </div>
      </footer>
    </>
  );
}
