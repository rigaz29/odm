// Context menu (Radix), in the manner of shadcn/ui's. It looks as the
// dropdown menu does (menu-styles).

import * as ContextMenuPrimitive from "@radix-ui/react-context-menu";
import { Check, ChevronRight } from "lucide-react";
import * as React from "react";
import { cn } from "../../lib/utils";
import { menuCheck, menuContent, menuItem, menuLabel, menuSeparator, menuShortcut } from "./menu-styles";

export const ContextMenu = ContextMenuPrimitive.Root;
export const ContextMenuTrigger = ContextMenuPrimitive.Trigger;
export const ContextMenuSub = ContextMenuPrimitive.Sub;

export function ContextMenuContent({ className, ...props }: React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.Content>) {
  return (
    <ContextMenuPrimitive.Portal>
      <ContextMenuPrimitive.Content className={cn(menuContent, className)} {...props} />
    </ContextMenuPrimitive.Portal>
  );
}

export function ContextMenuItem({ className, ...props }: React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.Item>) {
  return <ContextMenuPrimitive.Item className={cn(menuItem, className)} {...props} />;
}

export function ContextMenuCheckboxItem({ className, children, ...props }: React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.CheckboxItem>) {
  return (
    <ContextMenuPrimitive.CheckboxItem className={cn(menuItem, className)} {...props}>
      {children}
      <ContextMenuPrimitive.ItemIndicator className={menuCheck}>
        <Check />
      </ContextMenuPrimitive.ItemIndicator>
    </ContextMenuPrimitive.CheckboxItem>
  );
}

export function ContextMenuSubTrigger({ className, children, ...props }: React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.SubTrigger>) {
  return (
    <ContextMenuPrimitive.SubTrigger className={cn(menuItem, "data-[state=open]:bg-accent", className)} {...props}>
      {children}
      <ChevronRight className="ml-auto" />
    </ContextMenuPrimitive.SubTrigger>
  );
}

export function ContextMenuSubContent({ className, ...props }: React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.SubContent>) {
  return (
    <ContextMenuPrimitive.Portal>
      <ContextMenuPrimitive.SubContent className={cn(menuContent, className)} {...props} />
    </ContextMenuPrimitive.Portal>
  );
}

export function ContextMenuLabel({ className, ...props }: React.ComponentPropsWithoutRef<typeof ContextMenuPrimitive.Label>) {
  return <ContextMenuPrimitive.Label className={cn(menuLabel, className)} {...props} />;
}

export function ContextMenuSeparator() {
  return <ContextMenuPrimitive.Separator className={menuSeparator} />;
}

export function ContextMenuShortcut({ children }: { children: React.ReactNode }) {
  return <span className={menuShortcut}>{children}</span>;
}
