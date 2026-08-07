import type { Metadata } from "next";
import {
  Bricolage_Grotesque,
  Fira_Code,
  Manrope,
} from "next/font/google";
import "./globals.css";

const display = Bricolage_Grotesque({
  variable: "--font-display",
  subsets: ["latin"],
});

const body = Manrope({
  variable: "--font-body",
  subsets: ["latin"],
});

const mono = Fira_Code({
  variable: "--font-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Garden — Run Eve agents locally",
  description:
    "Run unmodified Eve agents with the official runtime, or choose Garden’s focused native Go runtime.",
  openGraph: {
    title: "Garden — Run Eve agents locally",
    description:
      "Two execution paths, one Eve-shaped project, and no hosted Garden service required.",
    type: "website",
  },
};

const directionContract = `
THESIS: Garden makes runtime ownership visible; refuse the generic centered SaaS hero and feature-card grid.
OWN-WORLD: Field-note parchment, forest ink, acid-leaf signals, botanical branching geometry, and precision tool labels.
STORY: Understand the two honest execution paths, inspect the protocol evidence, then install from source.
FIRST VIEWPORT: A left-anchored promise shares the frame with a branching runtime specimen; install is the primary action.
FORM: User-pinned Cultivated systems direction; seed key cultivated-systems-user-pin.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md
`;

/** Provides the root document, metadata, fonts, and auditable design contract. */
export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={`${display.variable} ${body.variable} ${mono.variable}`}>
      <body>
        <span
          className="design-contract"
          aria-hidden="true"
          dangerouslySetInnerHTML={{
            __html: `<!-- ${directionContract.replaceAll("--", "—")} -->`,
          }}
        />
        {children}
      </body>
    </html>
  );
}
