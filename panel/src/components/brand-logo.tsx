import Image from "next/image";

import { cn } from "@/lib/utils";

export function BrandLogo({ className }: { className?: string }) {
  return (
    <Image
      src="/brand-logo.svg"
      alt=""
      width={975}
      height={847}
      unoptimized
      className={cn("h-auto w-full", className)}
    />
  );
}
