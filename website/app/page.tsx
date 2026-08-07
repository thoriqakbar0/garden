import { CopyCommand } from "./copy-command";

const githubUrl = "https://github.com/thoriqakbar0/garden";

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
export default function Home() {
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

          <a className="nav-github" href={githubUrl} target="_blank" rel="noreferrer">
            GitHub <ExternalIcon />
          </a>
        </nav>
      </header>

      <main id="main-content">
        <section className="hero" id="top">
          <div className="hero-shell">
            <div className="hero-copy">
              <h1>Run Eve agents.<br /><span>Keep the runtime yours.</span></h1>
              <p className="hero-description">
                Garden runs an unmodified Eve project through the official runtime,
                or gives you a focused native Go path. Local by default. Explicit
                about the difference.
              </p>

              <div className="hero-actions">
                <a className="button button-primary" href="#install">
                  Install Garden <ArrowIcon />
                </a>
                <a className="button button-secondary" href="https://github.com/thoriqakbar0/garden/blob/main/COMPATIBILITY.md" target="_blank" rel="noreferrer">
                  Read compatibility
                </a>
              </div>

              <dl className="hero-facts" aria-label="Garden requirements">
                <div><dt>Official baseline</dt><dd>Eve 0.27.6</dd></div>
                <div><dt>Native runtime</dt><dd>Single Go process</dd></div>
                <div><dt>Hosted service</dt><dd>Not required</dd></div>
              </dl>
            </div>

            <div className="runtime-specimen" aria-label="One Eve project branching into official and native execution paths">
              <div className="specimen-header">
                <span>agent/weather</span>
                <span className="healthy"><i /> discovered</span>
              </div>

              <div className="source-bed">
                <span className="folder-tab">Eve-shaped project</span>
                <code>agent/</code>
                <ul>
                  <li>instructions.md</li>
                  <li>agent.ts</li>
                  <li>tools/get_weather.ts</li>
                </ul>
              </div>

              <div className="stem" aria-hidden="true"><i /><i /><i /></div>

              <div className="runtime-branches">
                <div className="branch branch-official">
                  <span className="branch-label">Full authored behavior</span>
                  <strong>official Eve</strong>
                  <code>--runtime eve</code>
                  <small>TypeScript stays TypeScript</small>
                </div>
                <div className="branch branch-native">
                  <span className="branch-label">Focused local contract</span>
                  <strong>Garden native</strong>
                  <code>default</code>
                  <small>One durable Go process</small>
                </div>
              </div>

              <div className="specimen-command">
                <span aria-hidden="true">$</span>
                <code>garden serve --runtime eve</code>
                <span className="cursor" aria-hidden="true" />
              </div>
            </div>
          </div>
        </section>

        <section className="runtime-section" id="runtime">
          <div className="section-shell">
            <div className="section-intro">
              <h2>Two honest paths through the same project.</h2>
              <p>
                Choose fidelity when authored Eve behavior matters. Choose the
                smaller native contract when a local Go runtime is the point.
                Garden never quietly swaps one for the other.
              </p>
            </div>

            <div className="runtime-landscape">
              <article className="mode mode-official">
                <h3>Your Eve project, still itself.</h3>
                <div className="mode-topline">
                  <span>Official mode</span><code>eve@0.27.6</code>
                </div>
                <p>
                  Garden supervises the pinned project-local runtime. Eve owns
                  compilation, tools, hooks, channels, subagents, schedules,
                  workflow semantics, and its sandbox.
                </p>
                <div className="mode-command"><code>garden serve --runtime eve</code></div>
                <ul aria-label="Official Eve mode characteristics">
                  <li>Authored TypeScript executes unchanged</li>
                  <li>Exact project-local runtime validation</li>
                  <li>Official protocol and session ownership</li>
                </ul>
              </article>

              <div className="mode-divider" aria-hidden="true">
                <span>same project</span>
              </div>

              <article className="mode mode-native">
                <h3>A smaller contract, deliberately.</h3>
                <div className="mode-topline">
                  <span>Native mode</span><code>garden</code>
                </div>
                <p>
                  Sessions, live streams, model and native-tool turns,
                  cancellation, and local recovery live in one Go process. No
                  arbitrary authored TypeScript is implied.
                </p>
                <div className="mode-command"><code>garden serve</code></div>
                <ul aria-label="Native Garden mode characteristics">
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
              <h2>The boundary is a live protocol, not a black box.</h2>
              <p>
                Garden’s native server exposes Eve-shaped HTTP sessions and
                protocol-v19 event streams. Continuations, replay, cancellation,
                and recovery remain inspectable from the client side.
              </p>
              <a href="https://github.com/thoriqakbar0/garden/blob/main/TESTING.md" target="_blank" rel="noreferrer">
                Inspect the test inventory <ArrowIcon />
              </a>
            </div>

            <div className="event-field" aria-label="Example Garden event stream">
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
            </div>
          </div>
        </section>

        <section className="principles-section" aria-labelledby="principles-title">
          <div className="section-shell principles-shell">
            <h2 id="principles-title">Built to stay legible when the agent does real work.</h2>
            <div className="principle-list">
              <article>
                <span className="principle-mark" aria-hidden="true" />
                <h3>Own the process</h3>
                <p>Loopback by default, bearer-protected beyond it, and no hosted Garden service between you and the runtime.</p>
              </article>
              <article>
                <span className="principle-mark" aria-hidden="true" />
                <h3>Keep the evidence</h3>
                <p>Durable local events, deterministic recovery, and executable compatibility tests replace hand-wavy parity claims.</p>
              </article>
              <article>
                <span className="principle-mark" aria-hidden="true" />
                <h3>Name the boundary</h3>
                <p>Official means delegated to Eve. Native means the supported Go contract. The interface tells you which one is running.</p>
              </article>
            </div>
          </div>
        </section>

        <section className="install-section" id="install">
          <div className="install-shell">
            <div className="install-copy">
              <h2>Plant it in your own environment.</h2>
              <p>
                Build Garden from source, install the binary, then point it at an
                Eve-shaped project. Nothing to sign up for.
              </p>
              <div className="requirement-row">
                <span>macOS or Linux</span>
                <span>Go 1.25+</span>
                <span>Node 24+ for official Eve</span>
              </div>
            </div>

            <div className="install-terminal">
              <div className="terminal-title"><span>Install from source</span><span>shell</span></div>
              <CopyCommand command="git clone https://github.com/thoriqakbar0/garden.git" />
              <div className="terminal-line"><span>$</span><code>cd garden</code></div>
              <div className="terminal-line"><span>$</span><code>make install</code></div>
              <div className="terminal-line terminal-result"><span>[ok]</span><code>~/.local/bin/garden</code></div>
            </div>
          </div>
        </section>

        <section className="closing-section">
          <div className="closing-shell">
            <div className="closing-mark" aria-hidden="true"><GardenMark /></div>
            <h2>Let Eve stay Eve.<br />Choose where Garden begins.</h2>
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
          <p>Local runtime infrastructure for Eve-shaped agents.</p>
          <div>
            <a href="https://github.com/thoriqakbar0/garden/blob/main/COMPATIBILITY.md" target="_blank" rel="noreferrer">Compatibility</a>
            <a href="https://github.com/thoriqakbar0/garden/blob/main/UPSTREAM.md" target="_blank" rel="noreferrer">Upstream</a>
            <a href={githubUrl} target="_blank" rel="noreferrer">Source</a>
          </div>
        </div>
      </footer>
    </>
  );
}
