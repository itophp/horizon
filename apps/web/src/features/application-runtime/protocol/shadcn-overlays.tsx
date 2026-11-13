import { useId, useState } from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandShortcut,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerTitle,
  DrawerTrigger,
} from "@/components/ui/drawer";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from "@/components/ui/hover-card";
import {
  Menubar,
  MenubarContent,
  MenubarMenu,
  MenubarTrigger,
} from "@/components/ui/menubar";
import {
  NavigationMenu,
  NavigationMenuContent,
  NavigationMenuItem,
  NavigationMenuList,
  NavigationMenuTrigger,
} from "@/components/ui/navigation-menu";
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { createToastManager, Toaster } from "@/components/ui/toast";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { toast } from "sonner";
import type {
  ComponentRenderer,
  ComponentRendererProps,
} from "./component-registry";
import { RuntimeMenuItems } from "./shadcn-menu-items";
import { labeledOptions, useBoundValue, useUI } from "./shadcn-props";

function useOverlay(p: ComponentRendererProps) {
  const ui = useUI(p);
  const [open, setOpen] = useBoundValue(p.props.open === true);
  const disabled = p.props.disabled === true;
  const side = ui.string("side");
  const placement: "top" | "bottom" | "left" | "right" | undefined =
    side === "top" || side === "bottom" || side === "left" || side === "right"
      ? side
      : undefined;
  return {
    ...ui,
    side: placement,
    open: !disabled && open,
    triggerDisabled: disabled,
    onOpenChange(next: boolean) {
      if (disabled) return;
      setOpen(next);
      ui.event("openChange", next);
    },
  };
}

export const overlayRenderers = {
  "alert-dialog": function Confirm(p) {
    const { label, title, description, disabled, text, t, event } = useUI(p);
    const [open, setOpen] = useBoundValue(p.props.open === true);
    function changeOpen(next: boolean) {
      if (disabled) return;
      setOpen(next);
      event("openChange", next);
      if (!next) event("cancel");
    }
    return (
      <AlertDialog open={open && !disabled} onOpenChange={changeOpen}>
        {p.props.hideTrigger !== true && (
          <AlertDialogTrigger
            disabled={disabled}
            render={<Button variant="outline" />}
          >
            {label}
          </AlertDialogTrigger>
        )}
        <AlertDialogContent>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
          {p.children}
          <AlertDialogFooter>
            <AlertDialogCancel>
              {text(p.props.cancelKey) ?? t("components.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={disabled}
              onClick={() => {
                setOpen(false);
                event("openChange", false);
                event("confirm");
              }}
            >
              {text(p.props.confirmKey)}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    );
  },
  command: function CommandListBox(p) {
    const { label, title, description, disabled, options, t, event, text } =
      useUI(p);
    const [open, setOpen] = useBoundValue(p.props.open === true);
    const dialog = p.props.mode === "dialog";
    const entries = Array.isArray(p.props.options)
      ? p.props.options.slice(0, 500)
      : [];
    const metadata = new Map<string, { group?: string; shortcut?: string }>();
    for (const entry of entries as unknown[]) {
      if (
        !entry ||
        typeof entry !== "object" ||
        !("value" in entry) ||
        typeof entry.value !== "string" ||
        metadata.has(entry.value)
      )
        continue;
      metadata.set(entry.value, {
        group: "groupKey" in entry ? text(entry.groupKey) : undefined,
        shortcut: "shortcutKey" in entry ? text(entry.shortcutKey) : undefined,
      });
    }
    const groups = new Map<string | undefined, typeof options>();
    for (const item of options) {
      const group = metadata.get(item.value)?.group;
      const items = groups.get(group);
      if (items) items.push(item);
      else groups.set(group, [item]);
    }
    const palette = (
      <Command label={label}>
        <CommandInput
          aria-label={label}
          disabled={disabled}
          placeholder={text(p.props.placeholderKey) ?? label}
        />
        <CommandList>
          <CommandEmpty>{t("components.noOptions")}</CommandEmpty>
          {Array.from(groups, ([heading, items]) => (
            <CommandGroup key={heading ?? ""} heading={heading}>
              {items.map((item) => (
                <CommandItem
                  key={item.value}
                  value={item.value}
                  keywords={[item.label]}
                  disabled={disabled || item.disabled}
                  onSelect={() => {
                    if (dialog) {
                      setOpen(false);
                      event("openChange", false);
                    }
                    event("select", item.value);
                  }}
                >
                  {item.label}
                  {metadata.get(item.value)?.shortcut && (
                    <CommandShortcut>
                      {metadata.get(item.value)?.shortcut}
                    </CommandShortcut>
                  )}
                </CommandItem>
              ))}
            </CommandGroup>
          ))}
        </CommandList>
      </Command>
    );
    if (!dialog) return palette;
    return (
      <Dialog
        open={open && !disabled}
        onOpenChange={(next) => {
          if (disabled) return;
          setOpen(next);
          event("openChange", next);
        }}
      >
        <DialogTrigger
          disabled={disabled}
          render={<Button variant="outline" />}
        >
          {label}
        </DialogTrigger>
        <DialogContent showCloseButton={false}>
          <DialogTitle className="sr-only">{title ?? label}</DialogTitle>
          {description && (
            <DialogDescription className="sr-only">
              {description}
            </DialogDescription>
          )}
          {palette}
        </DialogContent>
      </Dialog>
    );
  },
  "context-menu": function ContextActions(p) {
    const { label, disabled, event, t } = useUI(p);
    return (
      <ContextMenu>
        <ContextMenuTrigger aria-label={label}>
          {p.children ?? label}
        </ContextMenuTrigger>
        <ContextMenuContent>
          <RuntimeMenuItems
            items={p.props.options}
            variant="context"
            disabled={disabled}
            t={t}
            select={(value) => event("select", value)}
          />
        </ContextMenuContent>
      </ContextMenu>
    );
  },
  "dropdown-menu": function Menu(p) {
    const { label, disabled, event, t } = useUI(p);
    return (
      <DropdownMenu>
        <DropdownMenuTrigger render={<Button variant="outline" />}>
          {label}
        </DropdownMenuTrigger>
        <DropdownMenuContent>
          <RuntimeMenuItems
            items={p.props.options}
            variant="dropdown"
            disabled={disabled}
            t={t}
            select={(value) => event("select", value)}
          />
        </DropdownMenuContent>
      </DropdownMenu>
    );
  },
  drawer: function BottomDrawer(p) {
    const {
      label,
      title,
      description,
      t,
      side,
      open,
      onOpenChange,
      triggerDisabled,
    } = useOverlay(p);
    const direction = side === "top" ? "up" : side === "bottom" ? "down" : side;
    return (
      <Drawer
        open={open}
        onOpenChange={onOpenChange}
        swipeDirection={direction}
      >
        <DrawerTrigger
          disabled={triggerDisabled}
          render={<Button variant="outline" />}
        >
          {label}
        </DrawerTrigger>
        <DrawerContent>
          <DrawerTitle>{title}</DrawerTitle>
          <DrawerDescription>{description}</DrawerDescription>
          {p.children}
          <DrawerClose render={<Button variant="outline" />}>
            {t("components.close")}
          </DrawerClose>
        </DrawerContent>
      </Drawer>
    );
  },
  "hover-card": function HoverPreview(p) {
    const { label, description } = useUI(p);
    const { number, side } = useOverlay(p);
    const delay = number("delay");
    const closeDelay = number("closeDelay");
    return (
      <HoverCard>
        <HoverCardTrigger
          delay={
            delay === undefined ? undefined : Math.max(0, Math.min(5000, delay))
          }
          closeDelay={
            closeDelay === undefined
              ? undefined
              : Math.max(0, Math.min(5000, closeDelay))
          }
          render={<Button variant="link" nativeButton={true} />}
        >
          {label}
        </HoverCardTrigger>
        <HoverCardContent side={side}>
          {description && <p>{description}</p>}
          {p.children}
        </HoverCardContent>
      </HoverCard>
    );
  },
  menubar: function MenuBar(p) {
    const { label, disabled, event, t } = useUI(p);
    const menus = labeledOptions(p.props.menus, t, 20);
    const groups = new Map<string, unknown>();
    if (Array.isArray(p.props.menus)) {
      for (const entry of p.props.menus.slice(0, 20) as unknown[]) {
        if (
          entry &&
          typeof entry === "object" &&
          "value" in entry &&
          typeof entry.value === "string" &&
          "options" in entry &&
          !groups.has(entry.value)
        )
          groups.set(entry.value, entry.options);
      }
    }
    const items = Array.isArray(p.props.menus)
      ? menus.map((menu) => ({
          ...menu,
          options: groups.get(menu.value),
        }))
      : [
          {
            value: undefined,
            label,
            options: p.props.options,
            disabled: false,
          },
        ];
    return (
      <Menubar aria-label={label}>
        {items.map((menu) => (
          <MenubarMenu key={menu.value ?? "default"}>
            <MenubarTrigger disabled={menu.disabled}>
              {menu.label}
            </MenubarTrigger>
            <MenubarContent>
              <RuntimeMenuItems
                items={menu.options}
                variant="dropdown"
                disabled={disabled}
                t={t}
                select={(selection) =>
                  event(
                    "select",
                    menu.value === undefined
                      ? selection
                      : {
                          menu: menu.value,
                          ...(typeof selection === "string"
                            ? { value: selection }
                            : selection),
                        },
                  )
                }
              />
            </MenubarContent>
          </MenubarMenu>
        ))}
      </Menubar>
    );
  },
  "navigation-menu": function Navigation(p) {
    const { label } = useUI(p);
    return (
      <NavigationMenu aria-label={label}>
        <NavigationMenuList className="flex-wrap">
          {p.props.group === true ? (
            p.children
          ) : (
            <NavigationMenuItem>
              <NavigationMenuTrigger>{label}</NavigationMenuTrigger>
              <NavigationMenuContent>{p.children}</NavigationMenuContent>
            </NavigationMenuItem>
          )}
        </NavigationMenuList>
      </NavigationMenu>
    );
  },
  "navigation-menu-item": function NavigationItem(p) {
    const { label } = useUI(p);
    return (
      <NavigationMenuItem>
        <NavigationMenuTrigger disabled={p.props.disabled === true}>
          {label}
        </NavigationMenuTrigger>
        <NavigationMenuContent>{p.children}</NavigationMenuContent>
      </NavigationMenuItem>
    );
  },
  popover: function PopoverPanel(p) {
    const {
      label,
      title,
      description,
      side,
      open,
      onOpenChange,
      triggerDisabled,
    } = useOverlay(p);
    return (
      <Popover open={open} onOpenChange={onOpenChange}>
        <PopoverTrigger
          disabled={triggerDisabled}
          render={<Button variant="outline" />}
        >
          {label}
        </PopoverTrigger>
        <PopoverContent side={side}>
          {title && <PopoverTitle>{title}</PopoverTitle>}
          {description && (
            <PopoverDescription>{description}</PopoverDescription>
          )}
          {p.children}
        </PopoverContent>
      </Popover>
    );
  },
  sheet: function SideSheet(p) {
    const {
      label,
      title,
      description,
      t,
      side,
      open,
      onOpenChange,
      triggerDisabled,
    } = useOverlay(p);
    return (
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetTrigger
          disabled={triggerDisabled}
          render={<Button variant="outline" />}
        >
          {label}
        </SheetTrigger>
        <SheetContent side={side} showCloseButton={false}>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
          {p.children}
          <SheetClose render={<Button variant="outline" />}>
            {t("components.close")}
          </SheetClose>
        </SheetContent>
      </Sheet>
    );
  },
  tabs: function TabChoices(p) {
    const { label, options, event, value, disabled } = useUI(p);
    const [active, setActive] = useBoundValue(
      typeof value === "string" ? value : null,
    );
    const available = options.filter((option) => !option.disabled);
    return (
      <Tabs
        orientation={
          p.props.orientation === "vertical" ? "vertical" : "horizontal"
        }
        value={
          available.some((option) => option.value === active)
            ? active
            : (available[0]?.value ?? null)
        }
        onValueChange={(next) => {
          if (p.props.disabled === true) return;
          setActive(next);
          if (!disabled) event("change", next);
        }}
      >
        <TabsList
          aria-label={label}
          activateOnFocus={
            p.props.activation === undefined
              ? undefined
              : p.props.activation === "automatic"
          }
          variant={p.props.variant === "line" ? "line" : "default"}
        >
          {options.map(({ value, label, disabled }) => (
            <TabsTrigger
              key={value}
              value={value}
              disabled={disabled || p.props.disabled === true}
            >
              {label}
            </TabsTrigger>
          ))}
        </TabsList>
        {p.children}
      </Tabs>
    );
  },
  tooltip: function Tip(p) {
    const { label, description } = useUI(p);
    const { number, side } = useOverlay(p);
    const delay = number("delay");
    const closeDelay = number("closeDelay");
    return (
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger
            delay={
              delay === undefined
                ? undefined
                : Math.max(0, Math.min(5000, delay))
            }
            closeDelay={
              closeDelay === undefined
                ? undefined
                : Math.max(0, Math.min(5000, closeDelay))
            }
            render={<Button variant="outline" />}
          >
            {label}
          </TooltipTrigger>
          <TooltipContent side={side}>{description}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    );
  },
  toast: function ToastFeedback(p) {
    const { label, title, description, t, disabled, text } = useUI(p);
    const [manager] = useState(createToastManager);
    return (
      <Toaster toastManager={manager} closeLabel={t("components.close")}>
        <Button
          type="button"
          disabled={disabled}
          onClick={() => {
            if (p.props.mode !== "action") {
              manager.add({ title, description });
              return;
            }
            void p.emit("click", undefined, {
              onOutcome: (status) => {
                const success = status === "succeeded";
                const title = success
                  ? text(p.props.successKey)
                  : text(p.props.errorKey);
                if (title)
                  manager.add({ title, type: success ? "success" : "error" });
              },
            });
          }}
        >
          {label}
        </Button>
      </Toaster>
    );
  },
  sonner: function SonnerFeedback(p) {
    const { label, title, description, t, disabled, text } = useUI(p);
    const id = useId();
    return (
      <>
        <Sonner id={id} containerAriaLabel={t("components.notifications")} />
        <Button
          type="button"
          disabled={disabled}
          onClick={() => {
            if (p.props.mode !== "action") {
              toast(title, { toasterId: id, description });
              return;
            }
            void p.emit("click", undefined, {
              onOutcome: (status) => {
                const success = status === "succeeded";
                const title = success
                  ? text(p.props.successKey)
                  : text(p.props.errorKey);
                if (title)
                  (success ? toast.success : toast.error)(title, {
                    toasterId: id,
                  });
              },
            });
          }}
        >
          {label}
        </Button>
      </>
    );
  },
} satisfies Record<string, ComponentRenderer>;
