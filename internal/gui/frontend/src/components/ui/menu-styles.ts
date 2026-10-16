// The look the context menu and the dropdown menu share.

export const menuContent =
  "z-50 min-w-48 overflow-hidden rounded-lg border border-border bg-popover p-1 text-sm text-foreground shadow-lg outline-none";

export const menuItem =
  "relative flex h-8 cursor-default select-none items-center gap-2 rounded-md px-2 outline-none data-[disabled]:pointer-events-none data-[highlighted]:bg-accent data-[disabled]:opacity-45 [&_svg]:size-4 [&_svg]:shrink-0 [&_svg]:text-muted-foreground";

// A checkbox item's check mark, at the right end so the items' icons stay
// in one column.
export const menuCheck = "ml-auto flex items-center pl-4 [&_svg]:text-foreground";

export const menuLabel = "px-2 py-1.5 text-xs font-medium text-muted-foreground";

export const menuSeparator = "-mx-1 my-1 h-px bg-border";

export const menuShortcut = "ml-auto pl-4 text-xs text-muted-foreground";
