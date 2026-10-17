import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

// cn joins class names, the later Tailwind class winning a conflict (as
// shadcn/ui's components expect).
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
