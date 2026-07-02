import type { ReactNode, SVGProps } from "react";

type IconProps = SVGProps<SVGSVGElement>;

const wrap = (children: ReactNode, props: IconProps) => (
  <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" {...props}>
    {children}
  </svg>
);

export const IconDatabase = (p: IconProps) =>
  wrap(
    <>
      <ellipse cx="8" cy="3.5" rx="5.5" ry="2" />
      <path d="M2.5 3.5v9c0 1.1 2.46 2 5.5 2s5.5-.9 5.5-2v-9" />
      <path d="M2.5 8c0 1.1 2.46 2 5.5 2s5.5-.9 5.5-2" />
    </>,
    p,
  );

export const IconWand = (p: IconProps) =>
  wrap(
    <>
      <path d="M2 14L10 6" />
      <path d="M9 2l.6 1.4L11 4l-1.4.6L9 6l-.6-1.4L7 4l1.4-.6L9 2z" />
      <path d="M13 7l.4 1 1 .4-1 .4-.4 1-.4-1-1-.4 1-.4.4-1z" />
    </>,
    p,
  );

export const IconFilter = (p: IconProps) => wrap(<path d="M2 3h12l-4.5 5.5V13l-3 1.5V8.5L2 3z" strokeLinejoin="round" />, p);

export const IconGitMerge = (p: IconProps) =>
  wrap(
    <>
      <circle cx="4" cy="3.5" r="1.5" />
      <circle cx="4" cy="12.5" r="1.5" />
      <circle cx="12" cy="12.5" r="1.5" />
      <path d="M4 5v3a4 4 0 004 4h2.5" />
      <path d="M4 5v6" />
    </>,
    p,
  );

export const IconLayers = (p: IconProps) =>
  wrap(
    <>
      <path d="M8 2l6 3-6 3-6-3 6-3z" strokeLinejoin="round" />
      <path d="M2 8l6 3 6-3" />
      <path d="M2 11l6 3 6-3" />
    </>,
    p,
  );

export const IconOutput = (p: IconProps) =>
  wrap(
    <>
      <rect x="2" y="3" width="8" height="10" rx="1" />
      <path d="M11 6l3 2-3 2" />
    </>,
    p,
  );
