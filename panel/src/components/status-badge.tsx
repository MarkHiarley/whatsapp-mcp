import { Badge } from "@/components/ui/badge";
import { STATUS_BADGE_VARIANTS, STATUS_DOT_CLASSES, STATUS_LABELS, type TenantStatusValue } from "@/lib/status";
import { cn } from "@/lib/utils";

export function StatusBadge({ status, className }: { status: TenantStatusValue; className?: string }) {
  return (
    <Badge variant={STATUS_BADGE_VARIANTS[status]} className={cn("gap-1.5 capitalize", className)}>
      <span aria-hidden className={cn("size-1.5 rounded-full", STATUS_DOT_CLASSES[status])} />
      {STATUS_LABELS[status]}
    </Badge>
  );
}
