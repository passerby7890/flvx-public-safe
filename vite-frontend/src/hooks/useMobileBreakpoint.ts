import { useEffect, useState } from "react";

export const DEFAULT_MOBILE_BREAKPOINT = 768;

export const getResponsiveViewportWidth = (): number => {
  if (typeof window === "undefined") {
    return DEFAULT_MOBILE_BREAKPOINT + 1;
  }

  const innerWidth = Number(window.innerWidth || 0);
  const visualViewportWidth = Number(window.visualViewport?.width || 0);
  const screenWidth = Number(window.screen?.width || 0);
  const candidates = [innerWidth, visualViewportWidth, screenWidth].filter(
    (value) => Number.isFinite(value) && value > 0,
  );

  if (candidates.length === 0) {
    return DEFAULT_MOBILE_BREAKPOINT + 1;
  }

  return Math.min(...candidates);
};

export const useMobileBreakpoint = (
  breakpoint = DEFAULT_MOBILE_BREAKPOINT,
): boolean => {
  const [isMobile, setIsMobile] = useState(() => {
    if (typeof window === "undefined") {
      return false;
    }

    return getResponsiveViewportWidth() <= breakpoint;
  });

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const onResize = () => {
      setIsMobile(getResponsiveViewportWidth() <= breakpoint);
    };

    window.addEventListener("resize", onResize);
    window.visualViewport?.addEventListener("resize", onResize);

    return () => {
      window.removeEventListener("resize", onResize);
      window.visualViewport?.removeEventListener("resize", onResize);
    };
  }, [breakpoint]);

  return isMobile;
};
