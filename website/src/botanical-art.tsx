type BotanicalVariant =
  | "sprout"
  | "branches"
  | "seedPods"
  | "leafCluster"
  | "roots"
  | "bloom";

type BotanicalShape = Readonly<{
  d: string;
  kind: "stem" | "leaf" | "petal" | "seed";
}>;

const ARTWORKS = {
  sprout: [
    { kind: "stem", d: "M121 316C119 266 123 218 120 169C118 124 125 87 127 42" },
    { kind: "leaf", d: "M120 231C84 229 61 207 57 174C89 176 113 193 120 231Z" },
    { kind: "leaf", d: "M122 184C158 181 180 158 183 127C151 130 128 149 122 184Z" },
    { kind: "leaf", d: "M122 270C151 269 169 251 173 226C146 228 128 243 122 270Z" },
    { kind: "petal", d: "M127 74C102 62 99 38 111 20C132 31 139 51 127 74Z" },
    { kind: "petal", d: "M129 74C148 55 171 59 184 75C166 92 145 91 129 74Z" },
    { kind: "petal", d: "M126 75C112 97 89 96 74 82C90 62 111 60 126 75Z" },
  ],
  branches: [
    { kind: "stem", d: "M120 318C120 260 120 197 119 131C119 95 121 61 124 25" },
    { kind: "stem", d: "M119 214C99 176 73 151 36 132" },
    { kind: "stem", d: "M120 183C143 145 169 119 205 101" },
    { kind: "leaf", d: "M76 162C43 164 21 148 14 121C43 118 67 131 76 162Z" },
    { kind: "leaf", d: "M157 145C166 114 189 99 216 102C209 129 188 146 157 145Z" },
    { kind: "leaf", d: "M119 261C88 258 69 239 66 212C94 215 113 233 119 261Z" },
    { kind: "petal", d: "M123 57C102 42 104 20 119 7C137 23 139 43 123 57Z" },
    { kind: "petal", d: "M124 58C145 46 163 57 168 75C145 83 129 76 124 58Z" },
  ],
  seedPods: [
    { kind: "stem", d: "M20 292C69 261 83 220 111 184C140 146 176 124 219 112" },
    { kind: "stem", d: "M78 236C69 205 71 177 84 150" },
    { kind: "stem", d: "M130 162C128 128 137 98 155 75" },
    { kind: "seed", d: "M68 169C54 150 59 130 76 121C92 139 88 158 68 169Z" },
    { kind: "seed", d: "M85 153C81 129 96 113 116 114C119 138 104 151 85 153Z" },
    { kind: "seed", d: "M145 94C138 72 149 55 168 52C176 75 164 91 145 94Z" },
    { kind: "seed", d: "M164 133C172 109 191 100 211 108C203 132 184 140 164 133Z" },
    { kind: "leaf", d: "M106 191C81 190 65 176 61 154C85 155 101 169 106 191Z" },
  ],
  leafCluster: [
    { kind: "stem", d: "M119 315C119 244 119 169 120 84" },
    { kind: "stem", d: "M119 238C97 207 75 183 47 165" },
    { kind: "stem", d: "M120 208C143 178 165 155 194 139" },
    { kind: "leaf", d: "M120 142C88 133 75 107 82 79C111 90 127 112 120 142Z" },
    { kind: "leaf", d: "M76 190C43 194 21 176 17 147C48 146 70 161 76 190Z" },
    { kind: "leaf", d: "M163 171C171 139 195 124 223 129C214 159 192 174 163 171Z" },
    { kind: "leaf", d: "M119 272C91 267 76 248 78 224C104 229 119 247 119 272Z" },
  ],
  roots: [
    { kind: "stem", d: "M121 15C120 72 120 126 121 181C121 216 118 246 112 274" },
    { kind: "stem", d: "M113 273C87 282 63 295 43 315" },
    { kind: "stem", d: "M113 274C139 283 164 296 188 317" },
    { kind: "stem", d: "M112 274C99 292 92 306 89 320" },
    { kind: "stem", d: "M119 273C135 291 144 306 149 320" },
    { kind: "leaf", d: "M120 103C86 101 66 81 64 52C94 54 115 73 120 103Z" },
    { kind: "leaf", d: "M122 151C154 147 174 127 176 99C147 103 127 122 122 151Z" },
  ],
  bloom: [
    { kind: "stem", d: "M120 320C120 252 120 194 120 133" },
    { kind: "leaf", d: "M120 248C83 246 61 224 58 193C91 195 114 214 120 248Z" },
    { kind: "leaf", d: "M122 213C157 209 178 187 180 157C148 161 127 181 122 213Z" },
    { kind: "petal", d: "M120 137C96 111 99 83 120 65C141 84 144 111 120 137Z" },
    { kind: "petal", d: "M122 137C128 102 151 88 178 95C173 125 152 142 122 137Z" },
    { kind: "petal", d: "M122 139C154 126 178 139 186 164C158 176 134 166 122 139Z" },
    { kind: "petal", d: "M118 139C106 172 80 180 56 166C67 137 90 127 118 139Z" },
    { kind: "petal", d: "M118 137C88 143 68 126 65 101C94 96 114 109 118 137Z" },
    { kind: "seed", d: "M120 137C132 137 141 146 141 158C141 170 132 179 120 179C108 179 99 170 99 158C99 146 108 137 120 137Z" },
  ],
} satisfies Record<BotanicalVariant, readonly BotanicalShape[]>;

type BotanicalArtProps = Readonly<{
  variant: BotanicalVariant;
}>;

/** Decorative botanical artwork that grows with its section’s scroll progress. */
export function BotanicalArt({ variant }: BotanicalArtProps) {
  return (
    <svg
      className={`botanical-art botanical-${variant}`}
      viewBox="0 0 240 320"
      aria-hidden="true"
      focusable="false"
    >
      {ARTWORKS[variant].map((shape, index) => (
        <path
          className={`botanical-${shape.kind}`}
          d={shape.d}
          key={`${shape.kind}-${index}`}
          pathLength={1}
        />
      ))}
    </svg>
  );
}


/** Decorative vine that follows the runtime specimen’s outer border. */
export function SpecimenVine() {
  return (
    <svg
      className="specimen-vine"
      viewBox="0 0 600 590"
      preserveAspectRatio="none"
      aria-hidden="true"
      focusable="false"
    >
      <path
        className="specimen-vine-stem"
        pathLength={1}
        d="M70 568C26 544 18 496 25 436L25 126C25 62 56 25 122 22H474C535 22 571 55 574 116V446C574 512 548 548 498 566"
      />
      <path className="specimen-vine-leaf" d="M28 472C4 454 3 429 18 411C41 427 44 450 28 472Z" />
      <path className="specimen-vine-leaf" d="M27 376C11 360 -5 365 -13 381C2 394 17 390 27 376Z" />
      <path className="specimen-vine-leaf" d="M42 78C20 57 25 32 45 21C65 41 62 64 42 78Z" />
      <path className="specimen-vine-leaf" d="M523 566C536 545 554 542 568 553C557 572 541 576 523 566Z" />
      <path className="specimen-vine-leaf" d="M573 420C557 406 558 388 570 377C584 390 584 407 573 420Z" />
      <path className="specimen-vine-leaf" d="M558 95C580 75 578 49 559 35C539 54 540 78 558 95Z" />
      <circle className="specimen-vine-berry" cx="29" cy="324" r="5" />
      <circle className="specimen-vine-berry" cx="573" cy="303" r="5" />
    </svg>
  );
}
