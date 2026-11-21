import type { SVGProps } from "react";

import { cn } from "@/lib/utils";

/**
 * WhatsApp Bridge mark: a speech bubble whose lower edge is a bridge deck,
 * with the arch cut out as negative space. It stays legible down to 16px.
 */
export function BrandMark({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg
      viewBox="0 0 40 40"
      fill="none"
      aria-hidden="true"
      focusable="false"
      className={cn("size-6", className)}
      {...props}
    >
      <path
        fill="currentColor"
        fillRule="evenodd"
        clipRule="evenodd"
        d="M8 12h24a5 5 0 0 1 5 5v9a5 5 0 0 1-5 5H22l-6 4v-4H8a5 5 0 0 1-5-5v-9a5 5 0 0 1 5-5Zm6 16h3v-6a3 3 0 0 1 6 0v6h3v-6a6 6 0 0 0-12 0v6Z"
      />
    </svg>
  );
}
