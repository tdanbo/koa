import { useEffect, useState } from "react";

/** useMediaQuery tracks a CSS media query from JS. */
export function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches);

  useEffect(() => {
    const list = window.matchMedia(query);
    const update = () => setMatches(list.matches);
    update();
    list.addEventListener("change", update);
    return () => list.removeEventListener("change", update);
  }, [query]);

  return matches;
}

/**
 * railCollapseQuery is the breakpoint PRD §5.5 leaves open. Below it the rail
 * drops to its 58px icon-only form, which is what keeps Discover's row layout —
 * description, status text and the fixed 82px action column — from crowding at
 * the window's 920px minimum width.
 */
export const railCollapseQuery = "(max-width: 1039px)";
