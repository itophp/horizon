import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

// Presentation props, not a workflow DSL or API DTO. Map from the server projection.
// X6 owns ports, edges, positioning and selection. This only renders node content.
export function WorkflowNodeCard({
  title,
  capabilityLabel,
  statusLabel,
  pending,
  inspectLabel,
  onInspect,
}: {
  title: string;
  capabilityLabel: string;
  statusLabel: string;
  pending: boolean;
  inspectLabel: string;
  onInspect: () => void;
}) {
  return (
    <Card size="sm" className="w-64" aria-busy={pending}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <Badge variant="outline">{statusLabel}</Badge>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-sm text-muted-foreground">{capabilityLabel}</p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={(event) => {
            event.stopPropagation();
            onInspect();
          }}
        >
          {inspectLabel}
        </Button>
      </CardContent>
    </Card>
  );
}
