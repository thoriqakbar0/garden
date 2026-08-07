---
name: "Garden"
description: "Cultivated systems for local, user-owned Eve agent runtimes."
colors:
  field-paper: "oklch(0.973 0.018 95)"
  field-paper-deep: "oklch(0.932 0.032 99)"
  field-paper-bright: "oklch(0.992 0.007 95)"
  forest-ink: "oklch(0.225 0.04 147)"
  forest-ink-soft: "oklch(0.405 0.036 145)"
  forest: "oklch(0.285 0.085 148)"
  forest-deep: "oklch(0.205 0.055 150)"
  moss: "oklch(0.48 0.105 139)"
  acid-leaf: "oklch(0.82 0.155 116)"
  acid-leaf-soft: "oklch(0.9 0.085 112)"
  botanical-line: "oklch(0.315 0.045 145 / 0.18)"
  botanical-line-strong: "oklch(0.315 0.045 145 / 0.32)"
typography:
  display:
    fontFamily: "Bricolage Grotesque, sans-serif"
    fontSize: "clamp(4rem, 7.6vw, 6rem)"
    fontWeight: 650
    lineHeight: 0.95
    letterSpacing: "-0.04em"
  headline:
    fontFamily: "Bricolage Grotesque, sans-serif"
    fontSize: "clamp(2.7rem, 5.3vw, 4.8rem)"
    fontWeight: 650
    lineHeight: 1.02
    letterSpacing: "-0.04em"
  body:
    fontFamily: "Manrope, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.6
    letterSpacing: "normal"
  technical-label:
    fontFamily: "Fira Code, monospace"
    fontSize: "0.67rem"
    fontWeight: 600
    lineHeight: 1.4
    letterSpacing: "0.08em"
  action:
    fontFamily: "Manrope, sans-serif"
    fontSize: "0.9rem"
    fontWeight: 750
    lineHeight: 1.6
    letterSpacing: "normal"
rounded:
  sm: "0.65rem"
  md: "1rem"
  lg: "1.5rem"
components:
  button-primary:
    backgroundColor: "{colors.forest}"
    textColor: "{colors.field-paper-bright}"
    typography: "{typography.action}"
    rounded: "{rounded.sm}"
    padding: "0 1.2rem"
    height: "3.25rem"
  button-primary-hover:
    backgroundColor: "{colors.forest-deep}"
    textColor: "{colors.field-paper-bright}"
    typography: "{typography.action}"
    rounded: "{rounded.sm}"
    padding: "0 1.2rem"
    height: "3.25rem"
  button-secondary:
    backgroundColor: "transparent"
    textColor: "{colors.forest}"
    typography: "{typography.action}"
    rounded: "{rounded.sm}"
    padding: "0 1.2rem"
    height: "3.25rem"
  runtime-card-official:
    backgroundColor: "{colors.forest}"
    textColor: "{colors.field-paper-bright}"
    rounded: "{rounded.lg}"
  runtime-card-native:
    backgroundColor: "{colors.field-paper-deep}"
    textColor: "{colors.forest-ink}"
    rounded: "{rounded.lg}"
---

# Design System: Garden

## Overview

**Creative North Star: "Cultivated systems"**

Garden looks like precise runtime infrastructure recorded in a cultivated field notebook. Warm parchment surfaces and forest ink keep the page grounded; acid-leaf signals call attention to live state, action, and branching. Botanical imagery is meaning-bearing: each major section receives one illustration whose form mirrors its job, from the hero sprout and branching runtimes to protocol seed pods, install roots, and the closing bloom.

The composition is technical and restrained rather than generic SaaS. Its first viewport is deliberately asymmetric: a left-anchored promise shares the frame with a substantial branching runtime specimen. Behind both, licensed wheat-field footage moves through a Garden-colored ordered-dither shader, turning the product metaphor into a living field without reducing text contrast. Dense protocol and terminal details use the same visual world as the marketing copy, so evidence and product claims feel continuous.

**Key Characteristics:**
- Warm field-note parchment with deep forest ink
- Acid-leaf signals reserved for status, action, and branching
- Botanical forms used as information geometry
- Dithered wheat-field motion with a static reduced-motion fallback
- Asymmetric editorial layouts paired with precise runtime specimens
- Clear visual separation between official Eve ownership and Garden native ownership

## Colors

The palette pairs warm, lightly chromatic paper with near-black green ink, then uses restrained greens for structure and a high-energy leaf tone for signals.

### Primary
- **Forest**: The principal filled action and official-runtime surface; it carries light paper text and establishes Garden's technical botanical identity.
- **Deep Forest**: The darkest operational surface for protocol fields, terminals, and the footer; it is also the primary-button hover state.

### Secondary
- **Acid Leaf**: The high-salience signal for live state, branch nodes, bullets, selections, and terminal prompts.
- **Soft Acid Leaf**: A quiet native-runtime fill that keeps the alternate path distinct without implying the same ownership as official mode.
- **Moss**: Structural stems, file-tree rules, health dots, and branching connectors.

### Neutral
- **Field Paper**: The page canvas and default light surface.
- **Deep Field Paper**: Recessed beds, native-mode panels, and the installation section.
- **Bright Field Paper**: Light text on forest surfaces and the brightest raised specimens.
- **Forest Ink**: Default copy on paper.
- **Soft Forest Ink**: Secondary copy, navigation, captions, and supporting metadata.
- **Botanical Line**: Quiet dividers and field-note structure.
- **Strong Botanical Line**: Outlined controls and higher-emphasis separators.

## Typography

**Display Font:** Bricolage Grotesque (with sans-serif fallback)
**Body Font:** Manrope (with sans-serif fallback)
**Label/Mono Font:** Fira Code (with monospace fallback)

**Character:** Bricolage Grotesque gives the product voice a sturdy, cultivated irregularity without becoming rustic. Manrope keeps explanations direct and highly legible, while Fira Code makes runtime labels, commands, versions, streams, and requirements feel native to the developer workflow.

### Hierarchy
- **Display** (650, responsive display scale, 0.95 line-height): Used for the first-viewport promise; tightly tracked and balanced within a short measure.
- **Headline** (650, responsive section scale, 1.02 line-height): Used for major narrative turns and closing statements.
- **Title** (Bricolage Grotesque, 650, responsive by context): Used for runtime-mode and principle headings with compact line lengths.
- **Body** (400, base reading size, 1.6 line-height): Used for explanations, with restrained measures and softened ink for supporting copy.
- **Technical Label** (600, compact mono scale, 0.08em tracking, uppercase): Used for mode labels, specimen tabs, terminal headings, and other operational metadata.
- **Action** (750, compact body scale): Used for buttons and prominent links.

## Layout

The shared content shell is capped at 76rem and leaves 1.5rem at each inline edge. At and below 64rem it becomes a narrower fluid shell with 1.25rem edges; at and below 48rem the edges reduce to 1rem. Section spacing is generous and responsive, generally ranging from 6rem to 9rem vertically, so dense evidence panels have room to read.

Desktop compositions favor unequal two-column grids. The hero pairs a narrower copy column with a larger runtime specimen; later sections alternate explanatory copy with protocol or terminal evidence. The runtime comparison uses two equal cards separated by a narrow vertical branch. At 64rem the hero, protocol, and install grids stack. At 48rem navigation links collapse, the runtime comparison becomes a vertical sequence, and principle rows simplify. At 31rem primary and secondary hero actions share the available row, copy controls become full-width, and terminal content wraps safely for a 320px viewport.

Botanical branching is part of the grid rather than an overlay decoration. Lines, nodes, stems, and leaf marks connect related states and preserve reading order as the layout reflows.

## Elevation & Depth

The system combines tonal layering with restrained, diffuse shadows. Paper-depth changes establish most hierarchy; shadows are concentrated on interactive actions, the hero specimen, the authoritative official-runtime card, protocol evidence, and the install terminal. Dark operational surfaces also use faint inset lines so their internal structure remains legible.

### Shadow Vocabulary
- **Raised specimen** (`0 1.25rem 4rem oklch(0.23 0.045 145 / 0.12), 0 0.2rem 0.8rem oklch(0.23 0.045 145 / 0.08)`): The main light specimen floating above the hero field.
- **Primary action** (`0 0.65rem 1.5rem oklch(0.24 0.08 148 / 0.2), 0 0.15rem 0.4rem oklch(0.24 0.08 148 / 0.16)`): Resting depth for the filled primary button.
- **Official runtime** (`0 1.5rem 4rem oklch(0.22 0.07 148 / 0.2)`): Gives the official-runtime card stronger authority than the tonal native card.
- **Operational field** (`0 1.5rem 4rem oklch(0.08 0.025 150 / 0.35), inset 0 0 0 1px oklch(0.9 0.05 110 / 0.1)`): Separates dark stream evidence from the surrounding deep-forest section.

## Shapes

Corners are gently rounded at three established sizes: compact controls, commands, and the skip link use the small radius; nested project beds and runtime branches use the medium radius; major specimens, cards, fields, and terminals use the large radius. Pills are limited to small requirement tags.

Botanical silhouettes deliberately break the otherwise regular geometry. Brand marks use an uneven seed-like oval; principle marks and branch endpoints use rotated leaf forms; live status uses a compact circular dot with a soft ring. Connectors remain one-pixel stems and branches so the metaphor stays technical rather than illustrative.

## Components

### Buttons
- **Shape:** Compact rounded rectangle using the small radius and a 3.25rem minimum height.
- **Primary:** Bright paper text on forest, with 1.2rem inline padding and a compact directional icon where useful.
- **Hover / Focus:** Hover deepens the forest fill and expands the diffuse shadow. Keyboard focus uses a two-pixel current-color outline offset by four pixels. Active press scales to 0.96.
- **Secondary:** Forest text on a transparent field with an inset strong botanical line; hover introduces bright paper.

### Chips
- **Style:** Requirement tags use a fully pill-shaped, translucent bright-paper fill with soft-ink Fira Code text.
- **State:** These are static environment requirements, not selectable controls.

### Cards / Containers
- **Corner Style:** Major cards use the large radius; nested beds and branches use the medium radius.
- **Background:** Official mode uses forest with bright paper text. Native mode uses deep field paper with forest ink. Dark protocol and terminal containers use deep forest.
- **Shadow Strategy:** Official and operational evidence surfaces receive diffuse depth; native mode stays tonal and flat.
- **Border:** Dividers and dark-field inset edges use low-opacity botanical lines rather than prominent card strokes.
- **Internal Padding:** Major runtime cards use responsive padding from 1.6rem to 2.6rem; compact specimen branches use 1.2rem.

### Navigation
- **Style:** The brand combines a Bricolage Grotesque wordmark with a seed-shaped forest mark. Links are compact, semibold Manrope in soft ink and become forest on hover. The GitHub link remains forest and includes an external-link glyph.
- **Mobile:** Section links hide at 48rem while GitHub remains. Below 31rem its word label hides and the icon retains the control.

### Dithered Field
- **Style:** Locally hosted wheat footage is sampled frame by frame through a forest, leaf, and paper ordered-dither shader. A left-to-right paper mask protects headline contrast.
- **Lifecycle:** Rendering follows `requestVideoFrameCallback`, pauses outside the hero, owns WebGL cleanup, and falls back to a dithered poster frame when motion is reduced or playback is unavailable.
- **Attribution:** The adapted CC BY 3.0 footage and changes are documented in `THIRD_PARTY_NOTICES.md`.

### Runtime Branch Specimen
- **Style:** A bright-paper raised container shows one recessed Eve-shaped source bed, a moss stem, two runtime branches, and a quiet vine following its outside border.
- **Ownership:** The official branch is forest with bright text; the native branch is soft acid leaf with ink text. Their labels and commands stay explicit.
- **Motion:** Hero content enters in two restrained grouped settles. The command cursor blinks independently. Section illustrations use CSS view timelines to draw stems and unfurl leaves as their sections move through the viewport; reduced-motion users receive the complete static artwork.

### Event Stream
- **Style:** A deep-forest ordered list uses mono event names, tabular sequence numbers, quiet row separators, and a bright live-status dot.
- **Accent:** Only the active result row receives an acid-leaf wash and acid-leaf text.

### Install Terminal
- **Style:** A deep-forest terminal presents command rows with leaf-colored prompts and compact Fira Code text.
- **Copy Action:** The outlined copy control uses acid-leaf text; hover fills it with acid leaf and switches the text to deep forest. Its copied or failed status is announced through a polite live region.

## Do's and Don'ts

### Do:
- **Do** use warm paper depth changes before adding a shadow.
- **Do** reserve acid leaf for status, action, active evidence, and botanical connectors.
- **Do** make official Eve and Garden native ownership explicit in labels, commands, copy, and surface treatment.
- **Do** use stems, nodes, and leaf marks to explain relationships and runtime branching.
- **Do** give every major section one botanical composition whose form reflects that section’s meaning.
- **Do** preserve visible focus, reduced-motion behavior, and the documented 320px reflow.

### Don't:
- **Don't** center the first viewport into a generic SaaS hero or replace the runtime specimen with a feature-card grid.
- **Don't** add botanical imagery without a section-specific role or let it obscure product content.
- **Don't** style official and native modes as interchangeable or imply that native mode executes arbitrary authored TypeScript.
- **Don't** use acid leaf as a broad background or routine body-text color.
- **Don't** fabricate hosted-service, deployment, benchmark, customer, or compatibility claims.
