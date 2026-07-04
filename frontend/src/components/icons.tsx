import type { ReactNode, SVGProps } from "react";

// Minimal hand-rolled icon set (feather-style, 16x16, stroke-based) so the
// app has zero icon-library dependency. Add new icons here as needed rather
// than pulling in an entire icon package for a handful of glyphs.
type IconProps = SVGProps<SVGSVGElement>;

function base(children: ReactNode, props: IconProps) {
  return (
    <svg
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      {...props}
    >
      {children}
    </svg>
  );
}

export const IconHome = (p: IconProps) => base(<path d="M2 7l6-4.5L14 7v6.5a1 1 0 01-1 1H3a1 1 0 01-1-1V7z" />, p);
export const IconDatabase = (p: IconProps) =>
  base(
    <>
      <ellipse cx="8" cy="3.5" rx="5.5" ry="2" />
      <path d="M2.5 3.5v9c0 1.1 2.46 2 5.5 2s5.5-.9 5.5-2v-9" />
      <path d="M2.5 8c0 1.1 2.46 2 5.5 2s5.5-.9 5.5-2" />
    </>,
    p,
  );
export const IconWorkflow = (p: IconProps) =>
  base(
    <>
      <rect x="1.5" y="2" width="4" height="4" rx="1" />
      <rect x="10.5" y="2" width="4" height="4" rx="1" />
      <rect x="6" y="10" width="4" height="4" rx="1" />
      <path d="M3.5 6v2a2 2 0 002 2H6M12.5 6v2a2 2 0 01-2 2h-.5" />
    </>,
    p,
  );
export const IconShield = (p: IconProps) => base(<path d="M8 1.5l5.5 2v4c0 3.5-2.3 5.9-5.5 7-3.2-1.1-5.5-3.5-5.5-7v-4L8 1.5z" />, p);
export const IconUsers = (p: IconProps) =>
  base(
    <>
      <circle cx="5.5" cy="5" r="2.2" />
      <path d="M1.5 14c0-2.5 1.8-4 4-4s4 1.5 4 4" />
      <circle cx="11.5" cy="5.5" r="1.8" />
      <path d="M10 6.2c1.9.2 3 1.5 3.3 3.4" />
    </>,
    p,
  );
export const IconSun = (p: IconProps) =>
  base(
    <>
      <circle cx="8" cy="8" r="3" />
      <path d="M8 1.5v1.5M8 13v1.5M2.5 8H1M15 8h-1.5M3.5 3.5l1 1M11.5 11.5l1 1M3.5 12.5l1-1M11.5 4.5l1-1" />
    </>,
    p,
  );
export const IconMoon = (p: IconProps) => base(<path d="M13.5 9.5A5.5 5.5 0 016.5 2.5a5.7 5.7 0 00-.9-.1A6 6 0 108 14a6 6 0 005.5-3.6c-.4.1-.7.1-1 .1z" />, p);
export const IconMonitor = (p: IconProps) =>
  base(
    <>
      <rect x="1.5" y="2.5" width="13" height="8.5" rx="1" />
      <path d="M5.5 14h5M8 11v3" />
    </>,
    p,
  );
export const IconLogout = (p: IconProps) =>
  base(
    <>
      <path d="M6 14H3a1 1 0 01-1-1V3a1 1 0 011-1h3" />
      <path d="M10.5 11l3-3-3-3M13.3 8H6" />
    </>,
    p,
  );
export const IconPlus = (p: IconProps) => base(<path d="M8 2.5v11M2.5 8h11" />, p);
export const IconTrash = (p: IconProps) =>
  base(
    <>
      <path d="M2.5 4.5h11" />
      <path d="M5.5 4.5V3a1 1 0 011-1h3a1 1 0 011 1v1.5" />
      <path d="M4 4.5l.6 8.5a1 1 0 001 .9h4.8a1 1 0 001-.9l.6-8.5" />
    </>,
    p,
  );
export const IconPlay = (p: IconProps) => base(<path d="M4 2.5l9.5 5.5L4 13.5v-11z" strokeLinejoin="round" />, p);
export const IconCheck = (p: IconProps) => base(<path d="M3 8.5l3 3 7-7" />, p);
export const IconX = (p: IconProps) => base(<path d="M3.5 3.5l9 9M12.5 3.5l-9 9" />, p);
export const IconRefresh = (p: IconProps) =>
  base(
    <>
      <path d="M13.5 8A5.5 5.5 0 013 10.2M2.5 8A5.5 5.5 0 0113 5.8" />
      <path d="M2.5 3v3h3M13.5 13v-3h-3" />
    </>,
    p,
  );
export const IconChevronDown = (p: IconProps) => base(<path d="M4 6l4 4 4-4" />, p);
export const IconPlug = (p: IconProps) =>
  base(
    <>
      <path d="M5.5 1.5v3M10.5 1.5v3" />
      <path d="M3.5 4.5h9v2a4.5 4.5 0 01-4.5 4.5 4.5 4.5 0 01-4.5-4.5v-2z" />
      <path d="M8 11v3.5" />
    </>,
    p,
  );
export const IconActivity = (p: IconProps) => base(<path d="M1.5 8h3l2-5 3 10 2-5h3" />, p);
export const IconPanelLeft = (p: IconProps) =>
  base(
    <>
      <rect x="2" y="3" width="12" height="10" rx="1.5" />
      <path d="M6.5 3v10" />
    </>,
    p,
  );
export const IconSearch = (p: IconProps) =>
  base(
    <>
      <circle cx="7" cy="7" r="4.5" />
      <path d="M13.5 13.5L10.5 10.5" />
    </>,
    p,
  );
export const IconDownload = (p: IconProps) =>
  base(
    <>
      <path d="M8 1.5v8.5M4.5 6.5L8 10l3.5-3.5" />
      <path d="M2.5 12.5h11v2h-11z" />
    </>,
    p,
  );
export const IconClock = (p: IconProps) =>
  base(
    <>
      <circle cx="8" cy="8" r="6.2" />
      <path d="M8 4.8V8l2.6 1.5" />
    </>,
    p,
  );
export const IconSettings = (p: IconProps) =>
  base(
    <>
      <circle cx="8" cy="8" r="2.2" />
      <path d="M8 1.8v1.4M8 12.8v1.4M14.2 8h-1.4M3.2 8H1.8M12.3 3.7l-1 1M4.7 11.3l-1 1M12.3 12.3l-1-1M4.7 4.7l-1-1" />
    </>,
    p,
  );
