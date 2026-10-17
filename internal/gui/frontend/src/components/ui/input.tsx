// Text field and select, in the manner of shadcn/ui's. The select is the
// browser's own, styled: it opens the system's list, which the keyboard and
// Narrator know.

import { ChevronDown } from "lucide-react";
import * as React from "react";
import { cn } from "../../lib/utils";

const field =
  "h-8 w-full min-w-0 rounded-md border border-input bg-background px-2.5 text-sm outline-none transition-colors placeholder:text-muted-foreground focus:border-ring/70 focus:ring-2 focus:ring-ring/25 disabled:cursor-not-allowed disabled:opacity-50";

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(({ className, ...props }, ref) => (
  <input ref={ref} className={cn(field, className)} {...props} />
));
Input.displayName = "Input";

export function Select({ className, children, ...props }: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <span className={cn("relative block", className)}>
      <select className={cn(field, "appearance-none pr-8")} {...props}>
        {children}
      </select>
      <ChevronDown className="pointer-events-none absolute right-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
    </span>
  );
}
