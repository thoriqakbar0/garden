# Repository agent guidance

- Run `npx frog list` before starting work to see which project frictions are already known.
- Log project papercuts and friction in tooling, documentation, APIs, tests, or conventions as they occur with `npx frog log`.
- Do not log global, system, or internal friction.
- For UI work in `website/`, use Tailwind CSS as the default styling system. Migrate touched styles to Tailwind utilities; keep custom CSS only for effects or behavior that utilities cannot express clearly.
