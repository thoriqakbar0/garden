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

function EveMark() {
  return (
    <svg className="eve-credit-mark" viewBox="0 0 78 25" aria-hidden="true" focusable="false">
      <path d="M77.7002 3.89551H54.0762L37.5781 24.3818H32.3486L36.5322 19.1729L51.958 0H77.7002V3.89551ZM21.0898 24.3721H0V20.4766H21.0898V24.3721ZM77.7012 20.4766V24.3721H56.6104V20.4766H77.7012ZM17.7744 14.0537H0V10.1582H17.7744V14.0537ZM77.7012 14.0537H59.9268V10.1582H77.7012V14.0537ZM34.7197 3.89551H0V0H34.7197V3.89551Z" />
    </svg>
  );
}

function ArrowIcon() {
  return (
    <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
      <path d="M4 10h11M11 6l4 4-4 4" />
    </svg>
  );
}

function ExternalIcon() {
  return (
    <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
      <path d="M8 5H5v10h10v-3M11 5h4v4M15 5l-7 7" />
    </svg>
  );
}

/** Renders Garden’s primary marketing page. */
export function Home() {
  const ditherScale = useDitherScale();
  const [motionPaused, setMotionPaused] = useState(false);
  const heroStage = useHeroStage();

  return (
    <>
      <a className="skip-link" href="#main-content">
        Skip to content
      </a>

      <header className="site-header">
        <nav className="nav-shell" aria-label="Main navigation">
          <a className="brand" href="#top" aria-label="Garden home">
            <span className="brand-mark"><GardenMark /></span>
            <span>garden</span>
          </a>

          <div className="nav-links">
            <a href="#runtime">Runtime</a>
            <a href="#protocol">Protocol</a>
            <a href="#install">Install</a>
          </div>

          <button
            className="motion-toggle"
            type="button"
            data-paused={motionPaused}
            onClick={() => setMotionPaused((isPaused) => !isPaused)}
          >
            <span className="motion-toggle-icon" aria-hidden="true" />
            <span>{motionPaused ? "Play motion" : "Pause motion"}</span>
          </button>

          <a className="nav-github" href={githubUrl} target="_blank" rel="noreferrer">
            GitHub <ExternalIcon />
          </a>
        </nav>
      </header>

      <main id="main-content">
        <section className="hero" id="top">
          <DitheredField ditherScale={ditherScale} paused={motionPaused} />
          <div className="hero-shell">
            <div className={revealClassName("hero-copy", heroStage >= 1)}>
              <h1>
                <span className="hero-line hero-line-lead">
                  Run{" "}
                  <span className="hero-eve-product">
                    <a
                      className="hero-eve-credit"
                      href="https://vercel.com/eve"
                      target="_blank"
                      rel="noreferrer"
                      aria-label="Eve by Vercel"
                    >
                      <span className="hero-eve-word" aria-hidden="true">Eve</span>
                      <EveMark />
                      <span className="hero-eve-by">by Vercel <span aria-hidden="true">↗</span></span>
                    </a>
                  </span>.
                </span>
                <span className="hero-line hero-line-promise">Own the runtime.</span>
              </h1>
              <p className="hero-description">
                Garden runs your project with the official Eve runtime or the smaller
                Garden native runtime. Both run locally. Garden always shows
                which runtime is active.
              </p>

              <div className="hero-actions">
                <a className="button button-primary" href="#install">
                  Install Garden <ArrowIcon />
                </a>
                <a className="button button-secondary" href="https://github.com/thoriqakbar0/garden/blob/main/COMPATIBILITY.md" target="_blank" rel="noreferrer">
                  Review Eve compatibility
                </a>
              </div>

              <dl className="hero-facts" aria-label="Garden requirements">
                <div><dt>Official baseline</dt><dd>Eve 0.27.6</dd></div>
                <div><dt>Garden native</dt><dd>Single Go process</dd></div>
                <div><dt>Hosted service</dt><dd>Not required</dd></div>
              </dl>
            </div>

          </div>
        </section>

        <section className="runtime-section" id="runtime">
          <div className="section-shell">
            <div className="section-intro">
              <h2>One project. Two paths.</h2>
              <p>
                Choose the official Eve runtime for the full Eve feature set. Choose
                the Garden native runtime for a smaller local implementation.
                Garden always shows which runtime is active.
              </p>
            </div>

            <div className="runtime-landscape">
              <article className="mode mode-official">
                <h3>Eve stays Eve.</h3>
                <div className="mode-topline">
                  <span>Official Eve runtime</span><code>eve@0.27.6</code>
                </div>
                <p>
                  Garden supervises the pinned runtime. Eve keeps compilation,
                  tools, hooks, channels, subagents, schedules, workflows, and
                  sandboxing.
                </p>
                <div className="mode-command"><code>garden serve --runtime eve</code></div>
                <ul aria-label="Official Eve runtime characteristics">
                  <li>Authored TypeScript executes unchanged</li>
                  <li>Exact project-local runtime validation</li>
                  <li>Official protocol and session ownership</li>
                </ul>
              </article>

              <div className="mode-divider" aria-hidden="true">
                <span>same project</span>
              </div>

              <article className="mode mode-native">
                <h3>Less surface. On purpose.</h3>
                <div className="mode-topline">
                  <span>Garden native runtime</span><code>garden</code>
                </div>
                <p>
                  One Go process owns sessions, streams, model and native-tool
                  turns, cancellation, and recovery. It does not run arbitrary
                  authored TypeScript.
                </p>
                <div className="mode-command"><code>garden serve</code></div>
                <ul aria-label="Garden native runtime characteristics">
                  <li>Codex or OpenAI-compatible models</li>
                  <li>Fsync-backed local workflow history</li>
                  <li>Explicit native capability boundary</li>
                </ul>
              </article>
            </div>
          </div>
        </section>

        <section className="protocol-section" id="protocol">
          <div className="protocol-shell">
            <div className="protocol-copy">
              <h2>Watch the boundary.</h2>
              <p>
                Garden exposes Eve-shaped HTTP sessions and protocol-v19 event
                streams. Inspect continuations, replay, cancellation, and recovery
                from the client.
              </p>
              <a href="https://github.com/thoriqakbar0/garden/blob/main/TESTING.md" target="_blank" rel="noreferrer">
                Inspect the test inventory <ArrowIcon />
              </a>
            </div>

            <figure className="event-field">
              <figcaption className="sr-only">Example Garden event stream</figcaption>
              <div className="event-field-head">
                <span>session stream</span>
                <span><i /> live</span>
              </div>
              <ol>
                <li><span>001</span><code>session.started</code><small>ses_yk7</small></li>
                <li><span>002</span><code>turn.started</code><small>turn_01</small></li>
                <li><span>003</span><code>actions.requested</code><small>get_weather</small></li>
                <li className="event-accent"><span>004</span><code>action.result</code><small>72°F · sunny</small></li>
                <li><span>005</span><code>message.completed</code><small>assistant</small></li>
                <li><span>006</span><code>session.waiting</code><small>continuable</small></li>
              </ol>
            </figure>
          </div>
        </section>

        <section className="principles-section" aria-labelledby="principles-title">
          <div className="section-shell principles-shell">
            <div className="principles-intro">
              <h2 id="principles-title">Real work. Clear evidence.</h2>
              <DitheredTrees ditherScale={ditherScale} paused={motionPaused} />
            </div>
            <div className="principle-list">
              <article>
                <span className="principle-mark" aria-hidden="true" />
                <h3>Own the process</h3>
                <p>Loopback by default. Bearer-protected beyond it. No hosted Garden service in the middle.</p>
              </article>
              <article>
                <span className="principle-mark" aria-hidden="true" />
                <h3>Keep the evidence</h3>
                <p>Durable events. Deterministic recovery. Executable compatibility tests.</p>
              </article>
              <article>
                <span className="principle-mark" aria-hidden="true" />
                <h3>Name the boundary</h3>
                <p>The official Eve runtime runs the full Eve feature set. The Garden native runtime runs Garden’s supported Go feature set. Garden always shows which runtime is active.</p>
              </article>
            </div>
          </div>
        </section>

        <section className="install-section" id="install">
          <div className="install-shell">
            <div className="install-copy">
              <h2>Your machine. Your runtime.</h2>
              <p>
                Build Garden. Install the binary. Point it at an Eve-shaped
                project. No signup.
              </p>
              <div className="requirement-row">
                <span>macOS or Linux</span>
                <span>Go 1.25+</span>
                <span>Node 24+ for official Eve</span>
              </div>
            </div>

            <div className="install-terminal">
              <div className="terminal-title"><span>Install, then run native</span><span>shell</span></div>
              <div className="install-step">
                <span className="install-step-label"><i>1</i> Install Garden</span>
                <CopyCommand command="git clone https://github.com/thoriqakbar0/garden.git && cd garden && make install" />
              </div>
              <div className="install-step">
                <span className="install-step-label"><i>2</i> Run the Garden native runtime</span>
                <CopyCommand command="garden serve --root /path/to/eve-project" />
              </div>
              <div className="terminal-line terminal-result"><span>[live]</span><code>http://127.0.0.1:3000</code></div>
            </div>
          </div>
        </section>

        <section className="closing-section">
          <div className="closing-shell">
            <div className="closing-mark" aria-hidden="true"><GardenMark /></div>
            <h2>Eve stays Eve.<br />Garden starts where you choose.</h2>
            <div className="closing-actions">
              <a className="button button-primary" href={githubUrl} target="_blank" rel="noreferrer">
                View Garden on GitHub <ExternalIcon />
              </a>
              <a className="text-link" href="https://github.com/thoriqakbar0/garden/blob/main/README.md" target="_blank" rel="noreferrer">
                Read the documentation
              </a>
            </div>
          </div>
        </section>
      </main>

      <footer>
        <div className="footer-shell">
          <a className="brand" href="#top" aria-label="Back to Garden home">
            <span className="brand-mark"><GardenMark /></span>
            <span>garden</span>
          </a>
          <p className="footer-attribution">
            <span>Independent from Vercel and the Eve project.</span>
            <span>Built out of love for Eve’s open-source SDKs. 💚</span>
          </p>
          <div>
            <a href="https://github.com/thoriqakbar0/garden/blob/main/COMPATIBILITY.md" target="_blank" rel="noreferrer">Compatibility</a>
            <a href="https://github.com/thoriqakbar0/garden/blob/main/THIRD_PARTY_NOTICES.md" target="_blank" rel="noreferrer">Licenses</a>
            <a href={githubUrl} target="_blank" rel="noreferrer">Source</a>
          </div>
        </div>
      </footer>
    </>
  );
}
