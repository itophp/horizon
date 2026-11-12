import type { ReactNode } from "react";
import { EditorDialog } from "@/features/administration/shared";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet";
export function RuntimeDialogHost(props: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  presentation?: string;
  children: ReactNode;
}) {
  if (props.presentation !== "drawer" && props.presentation !== "sheet")
    return <EditorDialog {...props} wide />;
  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className="runtime-sheet">
        <SheetHeader>
          <SheetTitle>{props.title}</SheetTitle>
          <SheetDescription>{props.description}</SheetDescription>
        </SheetHeader>
        {props.children}
      </SheetContent>
    </Sheet>
  );
}
