export type DividerOrientation = "horizontal" | "vertical";

export interface DividerProps {
  orientation?: DividerOrientation;
}

/** Semantic separator. Renders as <hr> horizontally and as an ARIA-labelled
 * <div role="separator"> vertically (since <hr aria-orientation="vertical">
 * has poor cross-browser AT support). */
export function Divider({ orientation = "horizontal" }: DividerProps) {
  if (orientation === "vertical") {
    return <div role="separator" aria-orientation="vertical" className="divider-vertical" />;
  }
  return <hr className="divider" />;
}
